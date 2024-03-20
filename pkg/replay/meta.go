package replay

import (
	"bufio"
	"bytes"
	"github.com/3th1nk/easygo/util/strUtil"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	metaPrefix = ">>>>>>>> bytes per read >>>>>>>>"
)

func toMetaInfo(ns []int) bytes.Buffer {
	var meta = make([]string, 0, 8)
	var line = make([]string, 0, 1024)
	for i, n := range ns {
		if i > 0 && i%16 == 0 {
			meta = append(meta, strings.Join(line, " "))
			line = line[:0]
		}
		line = append(line, strconv.Itoa(n))
	}
	if len(line) > 0 {
		meta = append(meta, strings.Join(line, " "))
	}
	var builder bytes.Buffer
	builder.WriteString("\n\n\n\n")
	builder.WriteString(metaPrefix)
	builder.WriteString("\n")
	builder.WriteString(strings.Join(meta, "\n"))
	return builder
}

// parseMetaInfo 读取文件中的 meta 信息
func parseMetaInfo(data *os.File) ([]int, error) {
	cur, err := data.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = data.Seek(cur, io.SeekStart)
	}()

	var nBytes []int
	scanner := bufio.NewScanner(data)
	scanner.Split(bufio.ScanLines)
	var metaFound bool
	for scanner.Scan() {
		line := scanner.Text()
		if line == metaPrefix {
			metaFound = true
			continue
		}
		if metaFound && len(line) > 0 {
			ns, err := strUtil.SplitToInt(line, " ", true)
			if err != nil {
				return nil, err
			}
			nBytes = append(nBytes, ns...)
		}
	}
	return nBytes, nil
}
