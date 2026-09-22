package easyshell

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/3th1nk/easyshell/v2/internal/core"
	"github.com/3th1nk/easyshell/v2/internal/transfer"
	"github.com/pkg/sftp"
	"io"
	"os"
	"strings"
)

const defaultSftpMaxPacket = 1 << 15 // 32KiB

// TransferProtocol 文件传输协议
type TransferProtocol string

const (
	// ProtocolAuto 自动选择：优先 SFTP，设备不支持 SFTP 子系统时降级 SCP(默认)
	ProtocolAuto TransferProtocol = "auto"
	// ProtocolSftp 强制 SFTP(设备不支持时返回错误)
	ProtocolSftp TransferProtocol = "sftp"
	// ProtocolScp 强制 SCP
	ProtocolScp TransferProtocol = "scp"
)

// TransferOptions 文件传输的可选参数(可省略；最多传一个，传入则以传入为准)。
type TransferOptions struct {
	// Force 目标文件已存在时是否覆盖(目录拼接文件名后重新判断)
	Force bool
	// NoVerify 关闭上传后的自动校验(默认开启：上传完成后远端执行 md5sum 比对，
	//	设备不支持 md5sum 命令时自动跳过校验，不会导致上传失败)
	NoVerify bool
	// Protocol 传输协议，默认 ProtocolAuto
	Protocol TransferProtocol
	// Progress 传输进度回调(已传输字节数, 总字节数)。可为nil
	Progress func(transferred, total int64)
}

func firstTransferOpt(opts []TransferOptions) TransferOptions {
	if len(opts) > 0 {
		return opts[0]
	}
	return TransferOptions{}
}

// newSftpClient 创建一次性 sftp 客户端(调用方用完自行 Close)
func (s *SshShell) newSftpClient() (*sftp.Client, error) {
	cli, err := sftp.NewClient(s.client, sftp.MaxPacket(defaultSftpMaxPacket))
	if err != nil {
		return nil, &core.Error{Op: core.OpSftp, Addr: s.client.RemoteAddr().String(), Err: err}
	}
	return cli, nil
}

// sftpWithCtx 为 sftp 传输附加 context 取消能力：取消时关闭客户端解除阻塞
func (s *SshShell) sftpWithCtx(ctx context.Context, cli *sftp.Client, fn func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	errC := make(chan error, 1)
	go func() { errC <- fn() }()
	select {
	case err := <-errC:
		return err
	case <-ctx.Done():
		_ = cli.Close()
		<-errC
		return ctx.Err()
	}
}

// Upload 上传本地文件到远端。
//
//	传输协议自动选择(优先 SFTP，设备不支持 SFTP 子系统时自动降级 SCP，
//	可用 TransferOptions.Protocol 强制)；上传完成后自动校验(远端 md5sum 比对，
//	设备不支持 md5sum 命令时跳过校验)；
//	远端路径以 / 结尾时视为目录(自动拼接本地文件名)；目标已存在且未指定 Force 时返回 os.ErrExist；
//	写入临时文件后原子重命名，中途失败不会留下不完整的文件。
//	ctx 取消时中止传输。
func (s *SshShell) Upload(ctx context.Context, localPath, remotePath string, opts ...TransferOptions) error {
	opt := firstTransferOpt(opts)

	fi, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	useSftp := opt.Protocol != ProtocolScp
	var cli *sftp.Client
	if useSftp {
		c, err := s.newSftpClient()
		if err != nil {
			if opt.Protocol == ProtocolSftp {
				return err // 显式指定SFTP时不降级
			}
			useSftp = false
		} else {
			cli = c
			defer cli.Close()
		}
	}

	if useSftp {
		if err := s.sftpWithCtx(ctx, cli, func() error {
			return transfer.SftpUpload(cli, localPath, remotePath, transfer.Options{
				Force:    opt.Force,
				Progress: opt.Progress,
			})
		}); err != nil {
			return err
		}
		// 校验仅对单文件上传有意义；目录上传跳过校验
		if !opt.NoVerify && !fi.IsDir() {
			return s.verifyUploadedHash(ctx, remotePath, localPath)
		}
		return nil
	}

	return transfer.ScpUpload(ctx, s.client, localPath, remotePath, transfer.Options{
		Force:    opt.Force,
		Progress: opt.Progress,
	})
}

// Download 下载远端文件到本地(协议自动选择，同 Upload)。
func (s *SshShell) Download(ctx context.Context, remotePath, localPath string, opts ...TransferOptions) error {
	opt := firstTransferOpt(opts)

	cli, err := s.newSftpClient()
	if err != nil {
		// SFTP客户端创建失败(设备不支持SFTP子系统) → 降级SCP
		return transfer.ScpDownload(ctx, s.client, remotePath, localPath, transfer.Options{
			Force: opt.Force,
		})
	}
	defer cli.Close()
	if err = s.sftpWithCtx(ctx, cli, func() error {
		return transfer.SftpDownload(cli, remotePath, localPath, transfer.Options{
			Force:    opt.Force,
			Progress: opt.Progress,
		})
	}); err != nil {
		return err
	}
	return nil
}

// Delete 删除远端文件/目录(目录递归删除)。
//
//	仅支持具备 SFTP 子系统的设备；无 SFTP 的设备可用 s.Run(ctx, "rm -f ...") 代替。
func (s *SshShell) Delete(ctx context.Context, remotePath string) error {
	cli, err := s.newSftpClient()
	if err != nil {
		return err
	}
	defer cli.Close()
	return transfer.SftpRemove(cli, remotePath)
}

// verifyUploadedHash 上传后校验：远端执行 md5sum 与本地比对。
//
//	设备不支持 md5sum 命令时跳过校验(不视为错误)；哈希不一致返回错误。
func (s *SshShell) verifyUploadedHash(ctx context.Context, remotePath, localPath string) error {
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

	for _, line := range lines {
		for _, f := range strings.Fields(line) {
			if len(f) == 32 && isAllHex(f) {
				if !strings.Contains(line, localHash) {
					return &core.Error{Op: core.OpSftp, Err: fmt.Errorf("hash mismatch: local=%s remote-line=%q", localHash, line)}
				}
				return nil
			}
		}
	}
	// 输出中未找到哈希：设备不支持 md5sum，跳过校验
	return nil
}

// md5OfFile 计算本地文件MD5
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
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return len(s) > 0
}
