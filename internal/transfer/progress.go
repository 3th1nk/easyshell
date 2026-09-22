package transfer

import (
	"io"
	"os"
)

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

// sizer 可查询大小的文件(*os.File 与 *sftp.File 均满足)
type sizer interface {
	Stat() (os.FileInfo, error)
}
