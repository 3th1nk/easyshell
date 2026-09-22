package easyshell

import (
	"context"
	"fmt"
	"github.com/3th1nk/easyshell/v2/internal/core"
	"golang.org/x/crypto/ssh"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ScpOptions SCP 传输的可选参数(可省略)
type ScpOptions struct {
	// Force 目标文件已存在时是否覆盖(SCP 协议本身总是覆盖，此字段为与 SftpOptions 对齐保留)
	Force bool
	// Progress 传输进度回调(已传输字节数, 总字节数)。可为nil
	Progress func(transferred, total int64)
}

func firstScpOpt(opts []ScpOptions) ScpOptions {
	if len(opts) > 0 {
		return opts[0]
	}
	return ScpOptions{}
}

// ScpUpload 经 SCP 协议上传本地文件到远端。
//
//	适用于不支持 SFTP 子系统的老旧网络设备(走 exec 通道执行远端 scp 命令，远端需安装 scp)；
//	远端路径以 / 结尾时视为目录(自动拼接本地文件名)。
//	ctx 取消时中止传输(关闭会话解除阻塞)。
func (s *SshShell) ScpUpload(ctx context.Context, localPath, remotePath string, opts ...ScpOptions) error {
	opt := firstScpOpt(opts)
	fi, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if strings.HasSuffix(remotePath, "/") {
		remotePath += filepath.Base(localPath)
	}

	sess, err := s.client.NewSession()
	if err != nil {
		return &core.Error{Op: core.OpSession, Addr: s.client.RemoteAddr().String(), Err: err}
	}

	done := make(chan error, 1)
	go func() {
		defer sess.Close()
		done <- scpSink(sess, localPath, remotePath, fi.Size(), opt.Progress)
	}()
	return scpWait(ctx, sess, done)
}

// ScpDown 经 SCP 协议下载远端文件到本地。ctx 取消时中止传输。
func (s *SshShell) ScpDown(ctx context.Context, remotePath, localPath string, opts ...ScpOptions) error {
	opt := firstScpOpt(opts)

	sess, err := s.client.NewSession()
	if err != nil {
		return &core.Error{Op: core.OpSession, Addr: s.client.RemoteAddr().String(), Err: err}
	}

	done := make(chan error, 1)
	go func() {
		defer sess.Close()
		done <- scpSource(sess, remotePath, localPath, opt.Progress)
	}()
	return scpWait(ctx, sess, done)
}

// scpWait 等待传输完成；ctx 取消时关闭会话解除阻塞
func scpWait(ctx context.Context, sess *ssh.Session, done <-chan error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = sess.Close()
		<-done
		return &core.Error{Op: core.OpSftp, Err: ctx.Err()}
	}
}

// scpSink 上传：执行远端 "scp -t <path>" 并按 SCP 协议写入文件。
//
//	协议时序: 远端就绪(0x00) → C<size> <name>\n → 确认(0x00) → 文件内容 → 结束标记(0x00) → 远端确认(0x00)
func scpSink(sess *ssh.Session, localPath, remotePath string, size int64, progress func(transferred, total int64)) error {
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return err
	}

	if err = sess.Start("scp -t " + remotePath); err != nil {
		return &core.Error{Op: core.OpSftp, Err: err}
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	// 远端就绪
	if err = scpExpectOk(stdout, stderr); err != nil {
		return err
	}

	name := filepath.Base(localPath)
	if progress != nil {
		progress(0, size)
	}
	if _, err = fmt.Fprintf(stdin, "C0644 %d %s\n", size, name); err != nil {
		return err
	}
	if err = scpExpectOk(stdout, stderr); err != nil {
		return err
	}

	if progress != nil {
		if _, err = io.Copy(&progressWriter{w: stdin, total: size, fn: progress}, localFile); err != nil {
			return err
		}
	} else {
		if _, err = io.Copy(stdin, localFile); err != nil {
			return err
		}
	}
	if _, err = stdin.Write([]byte{0}); err != nil {
		return err
	}
	if err = scpExpectOk(stdout, stderr); err != nil {
		return err
	}
	return sess.Wait()
}

// scpSource 下载：执行远端 "scp -f <path>" 并按 SCP 协议读取文件。
//
//	协议时序: 起始标记(0x00) → 远端确认(0x00) → C<size> <name>\n → 就绪(0x00) → 文件内容 → 远端结束标记(0x00)
func scpSource(sess *ssh.Session, remotePath, localPath string, progress func(transferred, total int64)) error {
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return err
	}

	if err = sess.Start("scp -f " + remotePath); err != nil {
		return &core.Error{Op: core.OpSftp, Err: err}
	}

	// 起始标记并等待远端确认
	if _, err = stdin.Write([]byte{0}); err != nil {
		return err
	}
	if err = scpExpectOk(stdout, stderr); err != nil {
		return err
	}

	// 读取文件头: C<mode> <size> <name>(如 "C0644 22 a.bin")
	head, err := scpReadLine(stdout)
	if err != nil {
		return err
	}
	fields := strings.Fields(head)
	if len(fields) < 3 || len(fields[0]) < 2 || fields[0][0] != 'C' {
		return &core.Error{Op: core.OpSftp, Err: fmt.Errorf("bad scp header %q", head)}
	}
	size, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return &core.Error{Op: core.OpSftp, Err: fmt.Errorf("bad scp header %q: %v", head, err)}
	}
	// 就绪应答
	if _, err = stdin.Write([]byte{0}); err != nil {
		return err
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	if progress != nil {
		progress(0, size)
	}
	if _, err = io.Copy(localFile, io.LimitReader(stdout, size)); err != nil {
		return err
	}
	// 消耗远端结束标记
	var marker [1]byte
	if _, err = io.ReadFull(stdout, marker[:]); err != nil {
		return err
	}
	// 最终确认：数据已完整接收，远端可能已关闭会话，写入失败可忽略
	_, _ = stdin.Write([]byte{0})
	if progress != nil {
		progress(size, size)
	}
	return sess.Wait()
}

// scpExpectOk 读取应答字节：0=成功，1/2=远端错误(附带文本)
func scpExpectOk(r io.Reader, stderr io.Reader) error {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return err
	}
	switch buf[0] {
	case 0:
		return nil
	case 1, 2:
		line, _ := scpReadLine(r)
		errText, _ := io.ReadAll(stderr)
		return &core.Error{Op: core.OpSftp, Err: fmt.Errorf("scp: %s %s", line, strings.TrimSpace(string(errText)))}
	}
	return &core.Error{Op: core.OpSftp, Err: fmt.Errorf("scp: unexpected response %#x", buf[0])}
}

// scpReadLine 读取一行(到\n为止，不含\n)
func scpReadLine(r io.Reader) (string, error) {
	var line []byte
	var buf [1]byte
	for {
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return "", err
		}
		if buf[0] == '\n' {
			return strings.TrimRight(string(line), "\r"), nil
		}
		line = append(line, buf[0])
		if len(line) > 4096 {
			return "", fmt.Errorf("scp: line too long")
		}
	}
}
