package replay

import (
	"github.com/3th1nk/easygo/util"
	"os"
	"path/filepath"
)

type Writer struct {
	data  *os.File // 数据文件
	bytes []int    // 每次写入的字节数
}

func NewWriter(path string) *Writer {
	// 目录不存在则创建
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		util.PrintErrln("create data dir failed: %s", err)
		return nil
	}
	data, err := os.Create(path)
	if err != nil {
		util.PrintErrln("create data file failed: %s", err)
		return nil
	}

	return &Writer{data: data}
}

func (w *Writer) Close() error {
	if w.data != nil {
		mi := toMetaInfo(w.bytes)
		if _, err := w.data.Write(mi.Bytes()); err != nil {
			return err
		}
		if err := w.data.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) Write(p []byte) (n int, err error) {
	if w.data == nil {
		return 0, nil
	}
	n, err = w.data.Write(p)
	if err != nil {
		return
	}
	w.bytes = append(w.bytes, n)
	return
}
