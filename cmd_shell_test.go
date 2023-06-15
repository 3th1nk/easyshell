package easyshell

import (
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/internal/_test"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
	"time"
)

func TestCmdShell_1(t *testing.T) {
	s := NewCmdShell("ping www.baidu.com", nil)

	start := time.Now()
	var out []string
	err := s.ReadAll(time.Minute, func(lines []string) {
		out = append(out, lines...)
		for _, line := range lines {
			util.PrintTimeLn(line)
		}
	})
	if err == io.EOF {
		util.PrintTimeLn("-> EOF")
	} else if err != nil {
		util.PrintTimeLn("-> error: %v", err)
	}

	util.PrintTimeLn("End: took=%v", time.Since(start))

	assert.LessOrEqual(t, 8, len(out))
	assert.True(t, _test.HasLine(out, "ping statistics") || _test.HasLine(out, "Ping 统计信息"))
}

func TestCmdShell_2(t *testing.T) {
	s := NewCmdShell("cmd /K", nil)

	start := time.Now()
	var out []string
	for _, cmd := range []string{
		"c:",
		"dir",
	} {
		util.PrintTimeLn("======================================================= %v", cmd)
		s.Write(cmd)
		err := s.ReadToEndLine(time.Minute, func(lines []string) {
			out = append(out, lines...)
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		})
		if err == io.EOF {
			util.PrintTimeLn("-> EOF")
		} else if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}

	util.PrintTimeLn("End: took=%v", time.Since(start))

	assert.True(t, _test.HasLine(out, "个文件"))
	assert.True(t, _test.HasLine(out, "个目录"))
}
