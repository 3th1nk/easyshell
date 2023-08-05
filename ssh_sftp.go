package easyshell

import (
	"fmt"
	"github.com/3th1nk/easyshell/errors"
	"github.com/pkg/sftp"
	"os"
	"path/filepath"
)

func (this *SshShell) SftpClient(opt ...sftp.ClientOption) (*sftp.Client, error) {
	if this.sftp == nil {
		var err error
		if this.sftp, err = sftp.NewClient(this.client, opt...); err != nil {
			return nil, &errors.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: err}
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
				return &errors.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: fmt.Errorf("remote path is a directory")}
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

func (this *SshShell) Upload(localPath, remotePath string, force bool) error {
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

func (this *SshShell) downloadFile(cli *sftp.Client, remotePath, localPath string, force bool) error {

check:
	lfi, err := os.Stat(localPath)
	if !os.IsNotExist(err) {
		return err
	}
	if lfi != nil {
		if lfi.IsDir() {
			if filepath.Base(localPath) == filepath.Base(remotePath) {
				return &errors.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: fmt.Errorf("local path is a directory")}
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

func (this *SshShell) downloadDir(cli *sftp.Client, remotePath, localPath string, force bool) error {
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
			if err = this.downloadDir(cli, remoteFilePath, localFilePath, force); err != nil {
				return err
			}
		} else {
			if err = this.downloadFile(cli, remoteFilePath, localFilePath, force); err != nil {
				if !force && os.IsExist(err) {
					continue
				}
				return err
			}
		}
	}
	return nil
}

func (this *SshShell) Download(remotePath, localPath string, force bool) error {
	cli, err := this.SftpClient(sftp.MaxPacket(1 << 15))
	if err != nil {
		return err
	}

	rfi, err := os.Stat(remotePath)
	if err != nil {
		return err
	}
	if rfi.IsDir() {
		return this.downloadDir(cli, remotePath, localPath, force)
	}
	return this.downloadFile(cli, remotePath, localPath, force)
}
