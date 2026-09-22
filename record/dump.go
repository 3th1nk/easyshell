package record

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// DumpOptions Dump 的可选参数(可省略)
type DumpOptions struct {
	// Hex 以十六进制显示负载(默认输出可读文本，控制字符转义)
	Hex bool
	// NoRedact 不打码 secret 帧(默认打码为 ****)
	NoRedact bool
	// WallTime 显示墙钟时间(默认显示相对偏移)
	WallTime bool
}

// Dump 将录制文件转储为可读文本
func Dump(src io.Reader, dst io.Writer, opts ...DumpOptions) error {
	var opt DumpOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	meta, _, err := readHeader(src)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(dst)
	defer w.Flush()
	header := fmt.Sprintf("# %s v1 host=%s:%d proto=%s user=%s",
		magic, meta.Host, meta.Port, meta.Protocol, meta.User)
	if !meta.StartedAt.IsZero() {
		header += " start=" + meta.StartedAt.Format(timeLayout)
	}
	_, _ = fmt.Fprintln(w, header)

	for {
		frame, err := readFrame(src)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		payload := formatPayload(frame, opt)
		_, _ = fmt.Fprintf(w, "%13s  %s  %s\n", formatDelta(frame, opt), dirName(frame.Dir), payload)
	}
}

// DumpFile 将录制文件转储到目标文件
func DumpFile(srcPath, dstPath string, opts ...DumpOptions) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	return Dump(src, dst, opts...)
}

const timeLayout = "2006-01-02T15:04:05.000000Z07:00"

func formatDelta(f Frame, opt DumpOptions) string {
	return f.Delta.String()
}

func dirName(d Direction) string {
	switch d {
	case DirIn:
		return "in "
	case DirEvent:
		return "evt"
	default:
		return "out"
	}
}

func formatPayload(f Frame, opt DumpOptions) string {
	if f.Secret && !opt.NoRedact {
		return fmt.Sprintf("%dB  **** (secret)", len(f.Payload))
	}
	size := fmt.Sprintf("%dB", len(f.Payload))
	if opt.Hex {
		return size + "  " + hexString(f.Payload)
	}
	return size + "  " + strconvQuote(f.Payload)
}

// strconvQuote 可读文本：控制字符转义
func strconvQuote(p []byte) string {
	q := fmt.Sprintf("%q", string(p))
	return strings.TrimSuffix(strings.TrimPrefix(q, "\""), "\"")
}

// hexString 十六进制转储(每行16字节由调用方排版，这里单行)
func hexString(p []byte) string {
	const digits = "0123456789abcdef"
	var sb strings.Builder
	for i, b := range p {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteByte(digits[b>>4])
		sb.WriteByte(digits[b&0x0F])
	}
	return sb.String()
}
