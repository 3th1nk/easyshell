package record

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

// TestDumpAsciinema 导出格式与 secret 跳过
func TestDumpAsciinema(t *testing.T) {
	sk := &realSeeker{}
	w, err := NewWriter(sk, Meta{Host: "h", Protocol: "ssh", User: "admin"}, Options{CaptureInput: true})
	assert.NoError(t, err)
	_, _ = w.Write([]byte("banner\n"))
	_, _ = w.Input().Write([]byte("secret password\n"))
	assert.NoError(t, w.Close())

	var out bytes.Buffer
	assert.NoError(t, DumpAsciinema(bytes.NewReader(sk.Bytes()), &out))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	assert.Len(t, lines, 2, "应有header+1条输出事件(输入帧不导出): %q", lines)

	// header
	var header map[string]any
	assert.NoError(t, json.Unmarshal([]byte(lines[0]), &header))
	assert.Equal(t, float64(2), header["version"])
	assert.Equal(t, float64(80), header["width"])

	// 输出事件
	var ev1 []any
	assert.NoError(t, json.Unmarshal([]byte(lines[1]), &ev1))
	assert.Equal(t, "o", ev1[1])
	assert.Equal(t, "banner\n", ev1[2])

	// 输入帧(含敏感内容)不导出
	assert.NotContains(t, out.String(), "secret password")
}
