package replay

import (
	"github.com/3th1nk/easygo/util"
	"io"
	"os"
)

type Reader struct {
	bytes  []int // 每次读取的数据长度
	cursor int   // 当前读取的数据长度下标
	data   *os.File
}

func NewReader(path string) *Reader {
	data, err := os.Open(path)
	if err != nil {
		util.PrintTimeLn("open data file failed: %s", err)
		return nil
	}

	nBytes, err := parseMetaInfo(data)
	if err != nil {
		util.PrintTimeLn("parse meta info failed: %s", err)
		_ = data.Close()
		return nil
	}

	return &Reader{data: data, bytes: nBytes}
}

func (r *Reader) Close() error {
	if r.data != nil {
		return r.data.Close()
	}
	return nil
}

func (r *Reader) Read(p []byte) (n int, err error) {
	if len(r.bytes) == 0 {
		return r.data.Read(p)
	} else if r.cursor < len(r.bytes) {
		// 读取的数据长度不能超过 p 的长度
		bufLen, wantLen := len(p), r.bytes[r.cursor]
		if wantLen > bufLen {
			diff := wantLen - bufLen
			r.bytes[r.cursor] = diff
			wantLen = bufLen
		} else {
			r.cursor++
		}

		lr := io.LimitReader(r.data, int64(wantLen))
		return lr.Read(p)
	} else {
		return 0, io.EOF
	}
}
