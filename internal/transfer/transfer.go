// Package transfer 实现文件传输的协议细节(SFTP/SCP)与完整性辅助，由根包封装为 Shell 能力。
package transfer

import (
	"errors"
	"github.com/pkg/sftp"
	"io"
	"os"
	"path"
	"path/filepath"
)

var (
	// ErrRemoteIsDir 远端目标路径是目录
	ErrRemoteIsDir = errors.New("transfer: remote path is a directory")
	// ErrLocalIsDir 本地目标路径是目录
	ErrLocalIsDir = errors.New("transfer: local path is a directory")
)

// Options 传输的可选参数
type Options struct {
	// Force 目标文件已存在时是否覆盖(目录拼接文件名后重新判断)
	Force bool
	// Progress 传输进度回调(已传输字节数, 总字节数)。可为nil
	Progress func(transferred, total int64)
}

// progressReaderOrNil 返回带进度的 reader(progress 为 nil 时原样返回；total 未知时为 0)
func progressReaderOrNil(r io.Reader, total int64, progress func(transferred, total int64)) io.Reader {
	if progress == nil {
		return r
	}
	return &progressReader{r: r, total: total, fn: progress}
}

// progressWriterOrNil 返回带进度的 writer(progress 为 nil 时原样返回；total 未知时为 0)
func progressWriterOrNil(w io.Writer, total int64, progress func(transferred, total int64)) io.Writer {
	if progress == nil {
		return w
	}
	return &progressWriter{w: w, total: total, fn: progress}
}

// SftpUpload 经 SFTP 上传本地文件/目录到远端(目录递归上传)。
//
//	目标为目录时上传到目录下(以本地文件名命名)；目标文件已存在且未指定 Force 时返回 os.ErrExist。
//	单文件写入临时文件后重命名，避免中途失败留下不完整的文件。
//	远程路径必须用 path 包拼接：远程是类Unix系统用 '/' 分隔，
//	filepath 在 Windows 上会用 '\'，会被远程当成文件名字符。
func SftpUpload(cli *sftp.Client, localPath, remotePath string, opt Options) error {
	fi, err := os.Stat(localPath)
	if err != nil {
		println("DEBUG SftpUpload stat fail:", localPath, err.Error())
		return err
	}
	println("DEBUG SftpUpload entry:", localPath, "->", remotePath, "isdir:", fi.IsDir())
	if fi.IsDir() {
		if err := cli.MkdirAll(remotePath); err != nil {
			return err
		}
		entries, err := os.ReadDir(localPath)
		if err != nil {
			return err
		}
		for _, e := range entries {
			localEntry := filepath.Join(localPath, e.Name())
			remoteEntry := path.Join(remotePath, e.Name())
			if err := SftpUpload(cli, localEntry, remoteEntry, opt); err != nil {
				if !opt.Force && os.IsExist(err) {
					continue
				}
				return err
			}
		}
		return nil
	}
	return sftpUploadFile(cli, localPath, remotePath, opt.Force, opt.Progress)
}

// sftpUploadFile 上传单个文件。
//	目标为目录时拼接本地文件名后重新检查；已存在且不允许覆盖时返回 os.ErrExist。
func sftpUploadFile(cli *sftp.Client, localPath, remotePath string, force bool, progress func(transferred, total int64)) error {
	println("DEBUG sftpUploadFile:", localPath, "->", remotePath)
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
				return ErrRemoteIsDir
			}
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

	var total int64
	if fi, err := localFile.Stat(); err == nil {
		total = fi.Size()
	}

	// 原子写入：先写临时文件，成功后重命名(部分SFTP服务端不支持覆盖式rename，两级回退)
	tmpPath := remotePath + ".easyshell.tmp"
	tmpFile, err := cli.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err = tmpFile.ReadFrom(progressReaderOrNil(localFile, total, progress)); err != nil {
		_ = tmpFile.Close()
		_ = cli.Remove(tmpPath)
		return err
	}
	if err = tmpFile.Close(); err != nil {
		_ = cli.Remove(tmpPath)
		return err
	}
	if err = RenameRemote(cli, tmpPath, remotePath); err != nil {
		_ = cli.Remove(tmpPath)
		return err
	}
	return nil
}

// SftpDownload 经 SFTP 下载远端文件到本地。
//	本地路径为目录时下载到目录下(以远端文件名命名)；已存在且未指定 Force 时返回 os.ErrExist。
func SftpDownload(cli *sftp.Client, remotePath, localPath string, opt Options) error {
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
				return ErrLocalIsDir
			}
			localPath = filepath.Join(localPath, path.Base(remotePath))
			continue
		}
		if !opt.Force {
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

	var total int64
	if fi, err := remoteFile.Stat(); err == nil {
		total = fi.Size()
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	// WriteTo 使用 sftp 的并发读取优化
	_, err = remoteFile.WriteTo(progressWriterOrNil(localFile, total, opt.Progress))
	return err
}

// SftpRemove 删除远端文件/目录(目录递归删除)
func SftpRemove(cli *sftp.Client, remotePath string) error {
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
			if err = SftpRemove(cli, path.Join(remotePath, f.Name())); err != nil {
				return err
			}
		}
		return cli.RemoveDirectory(remotePath)
	}
	return cli.Remove(remotePath)
}

// RenameRemote 远端重命名：Posix-Rename → Rename(目标存在则先删) 两级回退
func RenameRemote(cli *sftp.Client, oldPath, newPath string) error {
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
