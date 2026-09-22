package easyshell

import (
	"fmt"
	"github.com/3th1nk/easyshell/core"
	"github.com/pkg/sftp"
	"os"
	"path"
	"path/filepath"
)

// SftpClient 获取sftp客户端，调用方无需Close
func (this *SshShell) SftpClient(opt ...sftp.ClientOption) (*sftp.Client, error) {
	if this.sftp == nil {
		var err error
		if this.sftp, err = sftp.NewClient(this.client, opt...); err != nil {
			return nil, &core.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: err}
		}
	}
	return this.sftp, nil
}

func (this *SshShell) uploadFile(cli *sftp.Client, localPath, remotePath string, force bool) error {
	// 远程路径已存在时的处理：目录则拼接文件名后重新检查；文件则按 force 决定是否覆盖
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
				return &core.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: fmt.Errorf("remote path is a directory")}
			}
			// 远程路径必须用 path 包拼接：远程是类Unix系统，用 '/' 分隔；
			// filepath 在 Windows 上会用 '\'，会被远程当成文件名字符
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

	remoteFile, err := cli.Create(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	_, err = remoteFile.ReadFrom(localFile)
	return err
}

func (this *SshShell) uploadDir(cli *sftp.Client, localPath, remotePath string, force bool) error {
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
			if err = this.uploadDir(cli, localFilePath, remoteFilePath, force); err != nil {
				return err
			}
		} else {
			if err = this.uploadFile(cli, localFilePath, remoteFilePath, force); err != nil {
				if !force && os.IsExist(err) {
					continue
				}
				return err
			}
		}
	}
	return nil
}

func (this *SshShell) SftpUpload(localPath, remotePath string, force bool) error {
	cli, err := this.SftpClient(sftp.MaxPacket(1 << 15))
	if err != nil {
		return err
	}

	fi, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		return this.uploadDir(cli, localPath, remotePath, force)
	}
	return this.uploadFile(cli, localPath, remotePath, force)
}

func (this *SshShell) downFile(cli *sftp.Client, remotePath, localPath string, force bool) error {
	// 本地路径已存在时的处理：目录则拼接文件名后重新检查；文件则按 force 决定是否覆盖
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
				return &core.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: fmt.Errorf("local path is a directory")}
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

	_, err = localFile.ReadFrom(remoteFile)
	return err
}

func (this *SshShell) downDir(cli *sftp.Client, remotePath, localPath string, force bool) error {
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
			if err = this.downDir(cli, remoteFilePath, localFilePath, force); err != nil {
				return err
			}
		} else {
			if err = this.downFile(cli, remoteFilePath, localFilePath, force); err != nil {
				if !force && os.IsExist(err) {
					continue
				}
				return err
			}
		}
	}
	return nil
}

func (this *SshShell) SftpDown(remotePath, localPath string, force bool) error {
	cli, err := this.SftpClient(sftp.MaxPacket(1 << 15))
	if err != nil {
		return err
	}

	rfi, err := cli.Stat(remotePath)
	if err != nil {
		return err
	}
	if rfi.IsDir() {
		return this.downDir(cli, remotePath, localPath, force)
	}
	return this.downFile(cli, remotePath, localPath, force)
}

// SftpRemove 删除远程文件、目录，如果是目录，则递归删除目录及子目录下的所有文件
func (this *SshShell) SftpRemove(remotePath string) error {
	cli, err := this.SftpClient(sftp.MaxPacket(1 << 15))
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
			if err = this.SftpRemove(path.Join(remotePath, f.Name())); err != nil {
				return err
			}
		}
		return cli.RemoveDirectory(remotePath)
	}
	return cli.Remove(remotePath)
}
