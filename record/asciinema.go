package record

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// AsciinemaOptions Asciinema 导出的可选参数(可省略)
type AsciinemaOptions struct {
	Width  int // 终端宽度，默认 80
	Height int // 终端高度，默认 24
}

// DumpAsciinema 将录制文件导出为 asciinema v2 格式(JSONL)。
//
//	导出后可直接用 asciinema 播放器回放：
//	  asciinema play session.cast
//	仅导出输出方向帧("o" 事件)——输入方向帧(命令/密码/应答)不导出，避免敏感信息进入导出文件。
func DumpAsciinema(src io.Reader, dst io.Writer, opts ...AsciinemaOptions) error {
	opt := AsciinemaOptions{Width: 80, Height: 24}
	if len(opts) > 0 {
		if opts[0].Width > 0 {
			opt.Width = opts[0].Width
		}
		if opts[0].Height > 0 {
			opt.Height = opts[0].Height
		}
	}

	meta, _, err := readHeader(src)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(dst)
	header := map[string]any{
		"version":   2,
		"width":     opt.Width,
		"height":    opt.Height,
		"timestamp": meta.StartedAt.Unix(),
		"env":       map[string]string{"SHELL": ""},
	}
	if err = writeJSONLine(w, header); err != nil {
		return err
	}

	for {
		frame, err := readFrame(src)
		if err == io.EOF {
			return w.Flush()
		}
		if err != nil {
			return err
		}
		if frame.Dir != DirOut || frame.Secret {
			continue
		}
		if err = writeJSONLine(w, []any{
			frame.Delta.Seconds(),
			"o",
			string(frame.Payload),
		}); err != nil {
			return err
		}
	}
}

func writeJSONLine(w *bufio.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
