package easyshell

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/3th1nk/easyshell/v2/internal/core"
	"github.com/pkg/sftp"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const defaultSftpMaxPacket = 1 << 15 // 32KiB

// SftpOptions SFTP 操作的可选参数(可省略，省略时使用默认值；传入则以传入值为准)
type SftpOptions struct {
	// Force 目标文件已存在时是否覆盖(目录拼接文件名后重新判断)
	Force bool
	// MaxPacket 单次传输的最大包大小，零值使用默认 32KiB
	MaxPacket uint32
	// HashVerify 上传完成后是否校验(远端执行 md5sum 比对，设备不支持该命令时会导致上传报错)
	HashVerify bool
	// Progress 传输进度回调(已传输字节数, 总字节数；远端文件总字节数未知时为0)。可为nil
	Progress func(transferred, total int64)
}

func (opt SftpOptions) clientOptions() []sftp.ClientOption {
	if opt.MaxPacket > 0 {
		return []sftp.ClientOption{sftp.MaxPacket(int(opt.MaxPacket))}
	}
	return []sftp.ClientOption{sftp.MaxPacket(defaultSftpMaxPacket)}
}

// SftpClient 获取 sftp 客户端(Shell 内缓存复用，随 Shell.Close 一起关闭，调用方无需 Close)。
func (s *SshShell) SftpClient(opt ...sftp.ClientOption) (*sftp.Client, error) {
	if s.sftpCli == nil {
		var err error
		if s.sftpCli, err = sftp.NewClient(s.client, opt...); err != nil {
			return nil, &core.Error{Op: core.OpSftp, Addr: s.client.RemoteAddr().String(), Err: err}
		}
	}
	return s.sftpCli, nil
}

// SftpUpload 上传本地文件/目录到远端。
//
//	目标为目录时上传到目录下(以本地文件/目录名命名)；目标文件已存在且未指定 Force 时返回 os.ErrExist。
//	ctx 取消时中止传输：关闭当前 sftp 连接(进行中的操作立即失败)，下次调用自动重建。
func (s *SshShell) SftpUpload(ctx context.Context, localPath, remotePath string, opts ...SftpOptions) error {
	opt := firstOpt(opts)
	cli, err := s.SftpClient(opt.clientOptions()...)
	if err != nil {
		return err
	}

	fi, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return s.sftpWithCtx(ctx, cli, func() error {
			return s.uploadDirProgress(cli, localPath, remotePath, opt.Force, opt.Progress)
		})
	}
	err = s.sftpWithCtx(ctx, cli, func() error {
		return s.uploadFileProgress(cli, localPath, remotePath, opt.Force, opt.Progress)
	})
	if err != nil {
		return err
	}
	if opt.HashVerify {
		return s.verifyRemoteHash(ctx, cli, localPath, remotePath)
	}
	return nil
}

// SftpDown 下载远端文件/目录到本地。
//
//	目标为目录时下载到目录下(以远端文件/目录名命名)；本地文件已存在且未指定 Force 时返回 os.ErrExist。
//	ctx 取消时中止传输：关闭当前 sftp 连接(进行中的操作立即失败)，下次调用自动重建。
func (s *SshShell) SftpDown(ctx context.Context, remotePath, localPath string, opts ...SftpOptions) error {
	opt := firstOpt(opts)
	cli, err := s.SftpClient(opt.clientOptions()...)
	if err != nil {
		return err
	}

	rfi, err := cli.Stat(remotePath)
	if err != nil {
		return err
	}
	if rfi.IsDir() {
		return s.sftpWithCtx(ctx, cli, func() error {
			return s.downDirProgress(cli, remotePath, localPath, opt.Force, opt.Progress)
		})
	}
	return s.sftpWithCtx(ctx, cli, func() error {
		return s.downFileProgress(cli, remotePath, localPath, opt.Force, opt.Progress)
	})
}

// sftpWithCtx 为 sftp 传输附加 context 取消能力：取消时关闭 sftp 连接中止传输，
//
//	连接缓存置空，下次调用自动重建。
func (s *SshShell) sftpWithCtx(ctx context.Context, cli *sftp.Client, fn func() error) error {
	if ctx == nil {
		return fn()
	}
	errC := make(chan error, 1)
	go func() { errC <- fn() }()
	select {
	case err := <-errC:
		return err
	case <-ctx.Done():
		s.sftpMu.Lock()
		if s.sftpCli == cli {
			_ = cli.Close()
			s.sftpCli = nil
		}
		s.sftpMu.Unlock()
		return ctx.Err()
	}
}

// SftpRemove 删除远端文件/目录(目录递归删除)
func (s *SshShell) SftpRemove(remotePath string) error {
	cli, err := s.SftpClient()
	if err != nil {
		return err
	}

	fi, err := cli.Stat(remotePath)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		fiArr, err := cli.ReadDir(remotePath)
		if err != nil {
			return err
		}
		// 不能直接删除非空目录，需要先删除其下的文件
		for _, f := range fiArr {
			if err = s.sftpRemove(cli, path.Join(remotePath, f.Name())); err != nil {
				return err
			}
		}
		return cli.RemoveDirectory(remotePath)
	}
	return cli.Remove(remotePath)
}

func (s *SshShell) sftpRemove(cli *sftp.Client, remotePath string) error {
	fi, err := cli.Stat(remotePath)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		fiArr, err := cli.ReadDir(remotePath)
		if err != nil {
			return err
		}
		for _, f := range fiArr {
			if err = s.sftpRemove(cli, path.Join(remotePath, f.Name())); err != nil {
				return err
			}
		}
		return cli.RemoveDirectory(remotePath)
	}
	return cli.Remove(remotePath)
}

// uploadFile 上传单个文件。
//
//	目标为目录时拼接本地文件名后重新检查；已存在且不允许覆盖时返回 os.ErrExist。
//	写入临时文件后重命名，避免中途失败留下不完整的文件。
func (s *SshShell) uploadFile(cli *sftp.Client, localPath, remotePath string, force bool) error {
	return s.uploadFileProgress(cli, localPath, remotePath, force, nil)
}

func (s *SshShell) uploadFileProgress(cli *sftp.Client, localPath, remotePath string, force bool, progress func(transferred, total int64)) error {
	for {
		rfi, err := cli.Stat(remotePath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if rfi == nil {
			break
		}
		if rfi.IsDir() {
			if filepath.Base(localPath) == path.Base(remotePath) {
				return &core.Error{Op: core.OpSftp, Addr: s.client.RemoteAddr().String(), Err: fmt.Errorf("remote path is a directory")}
			}
			// 远程路径必须用 path 包拼接：远程是类Unix系统用 '/' 分隔，
			//	filepath 在 Windows 上会用 '\'，会被远程当成文件名字符
			remotePath = path.Join(remotePath, filepath.Base(localPath))
			continue
		}
		if !force {
			return os.ErrExist
		}
		break
	}

	if err := cli.MkdirAll(path.Dir(remotePath)); err != nil {
		return err
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	var srcReader io.Reader = localFile
	if progress != nil {
		if fi, e := localFile.Stat(); e == nil {
			progress(0, fi.Size())
		}
		srcReader = &progressReader{r: localFile, total: progressTotal(localFile), fn: progress}
	}

	// 原子写入：先写临时文件，成功后重命名(部分SFTP服务端不支持覆盖式rename，三级回退)
	tmpPath := remotePath + ".easyshell.tmp"
	tmpFile, err := cli.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err = tmpFile.ReadFrom(srcReader); err != nil {
		_ = tmpFile.Close()
		_ = cli.Remove(tmpPath)
		return err
	}
	if err = tmpFile.Close(); err != nil {
		_ = cli.Remove(tmpPath)
		return err
	}
	if err = renameRemote(cli, tmpPath, remotePath); err != nil {
		_ = cli.Remove(tmpPath)
		return err
	}
	return nil
}

// renameRemote 远端重命名：Posix-Rename → Rename(目标存在则先删) 两级回退
func renameRemote(cli *sftp.Client, oldPath, newPath string) error {
	// openssh 等主流服务端支持 posix-rename@openssh.com 扩展(原子覆盖)
	if err := cli.PosixRename(oldPath, newPath); err == nil {
		return nil
	}
	// 标准 SFTP rename 要求目标不存在：先删除旧目标再重命名
	if _, err := cli.Stat(newPath); err == nil {
		if err := cli.Remove(newPath); err != nil {
			return err
		}
	}
	return cli.Rename(oldPath, newPath)
}

func (s *SshShell) uploadDir(cli *sftp.Client, localPath, remotePath string, force bool) error {
	return s.uploadDirProgress(cli, localPath, remotePath, force, nil)
}

func (s *SshShell) uploadDirProgress(cli *sftp.Client, localPath, remotePath string, force bool, progress func(transferred, total int64)) error {
	localFiles, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}

	if err = cli.MkdirAll(remotePath); err != nil {
		return err
	}

	for _, localFile := range localFiles {
		localFilePath := filepath.Join(localPath, localFile.Name())
		remoteFilePath := path.Join(remotePath, localFile.Name())
		if localFile.IsDir() {
			if err = s.uploadDirProgress(cli, localFilePath, remoteFilePath, force, progress); err != nil {
				return err
			}
		} else {
			if err = s.uploadFileProgress(cli, localFilePath, remoteFilePath, force, progress); err != nil {
				if !force && os.IsExist(err) {
					continue
				}
				return err
			}
		}
	}
	return nil
}

// progressReader 带进度回调的 reader
type progressReader struct {
	r           io.Reader
	total       int64
	transferred int64
	fn          func(transferred, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.transferred += int64(n)
		p.fn(p.transferred, p.total)
	}
	return n, err
}

func progressTotal(f *os.File) int64 {
	if fi, err := f.Stat(); err == nil {
		return fi.Size()
	}
	return 0
}

// downFile 下载单个文件。目标为目录时拼接远端文件名后重新检查；已存在且不允许覆盖时返回 os.ErrExist
func (s *SshShell) downFile(cli *sftp.Client, remotePath, localPath string, force bool) error {
	return s.downFileProgress(cli, remotePath, localPath, force, nil)
}

func (s *SshShell) downFileProgress(cli *sftp.Client, remotePath, localPath string, force bool, progress func(transferred, total int64)) error {
	for {
		lfi, err := os.Stat(localPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if lfi == nil {
			break
		}
		if lfi.IsDir() {
			if filepath.Base(localPath) == path.Base(remotePath) {
				return &core.Error{Op: core.OpSftp, Addr: s.client.RemoteAddr().String(), Err: fmt.Errorf("local path is a directory")}
			}
			localPath = filepath.Join(localPath, path.Base(remotePath))
			continue
		}
		if !force {
			return os.ErrExist
		}
		break
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	remoteFile, err := cli.Open(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	localFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	var dstWriter io.Writer = localFile
	if progress != nil {
		if rfi, e := cli.Stat(remotePath); e == nil {
			total := rfi.Size()
			progress(0, total)
			dstWriter = &progressWriter{w: localFile, total: total, fn: progress}
		}
	}

	// WriteTo 使用 sftp 的并发读取优化
	_, err = remoteFile.WriteTo(dstWriter)
	return err
}

// progressWriter 带进度回调的 writer
type progressWriter struct {
	w           io.Writer
	total       int64
	transferred int64
	fn          func(transferred, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	if n > 0 {
		p.transferred += int64(n)
		p.fn(p.transferred, p.total)
	}
	return n, err
}

func (s *SshShell) downDir(cli *sftp.Client, remotePath, localPath string, force bool) error {
	return s.downDirProgress(cli, remotePath, localPath, force, nil)
}

func (s *SshShell) downDirProgress(cli *sftp.Client, remotePath, localPath string, force bool, progress func(transferred, total int64)) error {
	remoteFiles, err := cli.ReadDir(remotePath)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(localPath, 0755); err != nil {
		return err
	}

	for _, remoteFile := range remoteFiles {
		remoteFilePath := path.Join(remotePath, remoteFile.Name())
		localFilePath := filepath.Join(localPath, remoteFile.Name())
		if remoteFile.IsDir() {
			if err = s.downDirProgress(cli, remoteFilePath, localFilePath, force, progress); err != nil {
				return err
			}
		} else {
			if err = s.downFileProgress(cli, remoteFilePath, localFilePath, force, progress); err != nil {
				if !force && os.IsExist(err) {
					continue
				}
				return err
			}
		}
	}
	return nil
}

func firstOpt(opts []SftpOptions) SftpOptions {
	if len(opts) > 0 {
		return opts[0]
	}
	return SftpOptions{}
}

// verifyRemoteHash 上传后校验：远端执行 md5sum 与本地比对
func (s *SshShell) verifyRemoteHash(ctx context.Context, cli *sftp.Client, localPath, remotePath string) error {
	localHash, err := md5OfFile(localPath)
	if err != nil {
		return err
	}

	var lines []string
	if err = s.Run(ctx, "md5sum "+remotePath, func(arr []string) {
		lines = append(lines, arr...)
	}); err != nil {
		return err
	}

	// 解析远端 md5sum 输出：<hash>  <path>(可能含回显前缀)
	for _, line := range lines {
		fields := strings.Fields(line)
		for i, f := range fields {
			if len(f) == 32 && isAllHex(f) {
				_ = i
				if !strings.Contains(line, localHash) {
					return &core.Error{Op: core.OpSftp, Err: fmt.Errorf("hash mismatch: local=%s remote-line=%q", localHash, line)}
				}
				return nil
			}
		}
	}
	return &core.Error{Op: core.OpSftp, Err: fmt.Errorf("md5sum output not parsed")}
}

func md5OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func isAllHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return len(s) > 0
}
