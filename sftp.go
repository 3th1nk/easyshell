package easyshell

import (
	"fmt"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/pkg/sftp"
	"os"
	"path"
	"path/filepath"
)

const defaultSftpMaxPacket = 1 << 15 // 32KiB

// SftpOptions SFTP 操作的可选参数(可省略，省略时使用默认值；传入则以传入值为准)
type SftpOptions struct {
	// Force 目标文件已存在时是否覆盖(目录拼接文件名后重新判断)
	Force bool
	// MaxPacket 单次传输的最大包大小，零值使用默认 32KiB
	MaxPacket uint32
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
func (s *SshShell) SftpUpload(localPath, remotePath string, opts ...SftpOptions) error {
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
		return s.uploadDir(cli, localPath, remotePath, opt.Force)
	}
	return s.uploadFile(cli, localPath, remotePath, opt.Force)
}

// SftpDown 下载远端文件/目录到本地。
//
//	目标为目录时下载到目录下(以远端文件/目录名命名)；本地文件已存在且未指定 Force 时返回 os.ErrExist。
func (s *SshShell) SftpDown(remotePath, localPath string, opts ...SftpOptions) error {
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
		return s.downDir(cli, remotePath, localPath, opt.Force)
	}
	return s.downFile(cli, remotePath, localPath, opt.Force)
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

	// 原子写入：先写临时文件，成功后重命名(部分SFTP服务端不支持覆盖式rename，三级回退)
	tmpPath := remotePath + ".easyshell.tmp"
	tmpFile, err := cli.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err = tmpFile.ReadFrom(localFile); err != nil {
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
			if err = s.uploadDir(cli, localFilePath, remoteFilePath, force); err != nil {
				return err
			}
		} else {
			if err = s.uploadFile(cli, localFilePath, remoteFilePath, force); err != nil {
				if !force && os.IsExist(err) {
					continue
				}
				return err
			}
		}
	}
	return nil
}

// downFile 下载单个文件。目标为目录时拼接远端文件名后重新检查；已存在且不允许覆盖时返回 os.ErrExist
func (s *SshShell) downFile(cli *sftp.Client, remotePath, localPath string, force bool) error {
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

	// WriteTo 使用 sftp 的并发读取优化
	_, err = remoteFile.WriteTo(localFile)
	return err
}

func (s *SshShell) downDir(cli *sftp.Client, remotePath, localPath string, force bool) error {
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
			if err = s.downDir(cli, remoteFilePath, localFilePath, force); err != nil {
				return err
			}
		} else {
			if err = s.downFile(cli, remoteFilePath, localFilePath, force); err != nil {
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
