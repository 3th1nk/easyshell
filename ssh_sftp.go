package easyshell

import (
	"fmt"
	"github.com/3th1nk/easyshell/core"
	"github.com/pkg/sftp"
	"os"
	"path/filepath"
)

// Sftp 获取sftp客户端，调用方无需Close
func (this *SshShell) Sftp(opt ...sftp.ClientOption) (*sftp.Client, error) {
	if this.sftp == nil {
		var err error
		if this.sftp, err = sftp.NewClient(this.client, opt...); err != nil {
			return nil, &core.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: err}
		}
	}
	return this.sftp, nil
}

func (this *SshShell) uploadFile(cli *sftp.Client, localPath, remotePath string, force bool) error {

check:
	rfi, err := cli.Stat(remotePath)
	if !os.IsNotExist(err) {
		return err
	}

	if rfi != nil {
		if rfi.IsDir() {
			if filepath.Base(localPath) == filepath.Base(remotePath) {
				return &core.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: fmt.Errorf("remote path is a directory")}
			}
			remotePath = filepath.Join(remotePath, filepath.Base(localPath))
			goto check
		}

		if !force {
			return os.ErrExist
		}
	}

	if err = cli.MkdirAll(filepath.Dir(remotePath)); err != nil {
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
		remoteFilePath := filepath.Join(remotePath, localFile.Name())
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
	cli, err := this.Sftp(sftp.MaxPacket(1 << 15))
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

check:
	lfi, err := os.Stat(localPath)
	if !os.IsNotExist(err) {
		return err
	}
	if lfi != nil {
		if lfi.IsDir() {
			if filepath.Base(localPath) == filepath.Base(remotePath) {
				return &core.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: fmt.Errorf("local path is a directory")}
			}
			localPath = filepath.Join(localPath, filepath.Base(remotePath))
			goto check
		}

		if !force {
			return os.ErrExist
		}
	}

	if err = os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
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
		remoteFilePath := filepath.Join(remotePath, remoteFile.Name())
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
	cli, err := this.Sftp(sftp.MaxPacket(1 << 15))
	if err != nil {
		return err
	}

	rfi, err := os.Stat(remotePath)
	if err != nil {
		return err
	}
	if rfi.IsDir() {
		return this.downDir(cli, remotePath, localPath, force)
	}
	return this.downFile(cli, remotePath, localPath, force)
}

// SftpRemove 删除远程文件、目录，如果是目录，则递归删除目录及子目录下的所有文件
func (this *SshShell) SftpRemove(path string) error {
	cli, err := this.Sftp(sftp.MaxPacket(1 << 15))
	if err != nil {
		return err
	}
	fi, err := cli.Stat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		fiArr, err := cli.ReadDir(path)
		if err != nil {
			return err
		}
		// 不能直接删除非空目录，需要先删除其下的文件
		for _, f := range fiArr {
			if err = this.SftpRemove(filepath.Join(path, f.Name())); err != nil {
				return err
			}
		}
		return cli.RemoveDirectory(path)
	}
	return cli.Remove(path)
}
