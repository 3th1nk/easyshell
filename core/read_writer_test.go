package core

import (
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
	"time"
)

func TestDefaultPromptRegex(t *testing.T) {
	var rw ReadWriter
	for _, obj := range []struct {
		Prompt string
		Expect bool
	}{
		{"root@test-01 $", true},
		{"root@test-01 #", true},
		{"root@test-01~ $", true},
		{"root@test-01~ # ", true},
		{"root@test-01(active)>", true},
		{"root@test-01(active)> ", true},
		{"root@test-01(M)]", true},
		{"root@test-01(S)] ", true},
		{"root@test-01(active) ] ", true},
		{"root@test-01(active)% ", true},
		{"[root@test-01 ~]#", true},
		{"<A10-8&A10-7_CSZW-Core-Switch>", true},
		{"S-DGB1-H17-WZJR-~(M)# ", true},
		{"S-DGB1-H17-WZJR-~(B)# ", true},
		{"中文名称 #", true},
		{"(CN-SZ-MC01) *#", true},
		{"mtk54007@(szimhM)(cfg-sync Standalone)(Active)(/Common)(tmos)#", true},
		{"root@test-01(active)a ", false},
		{"]", false},
		{"#", false},
		{"$", false},
		{" # ", false},
		{"[mon@m41205302.cloud.208.am49 /home/mon]", true},
		{"[testuser@localhost ~]$ Login:", false},
		{"[testuser@localhost ~]$ Username:", false},
		{"[testuser@localhost ~]$ Password:", false},
	} {
		if obj.Expect != rw.IsEndLine(obj.Prompt) {
			t.Error(obj.Prompt)
		}
	}
}

func TestFindHostname(t *testing.T) {
	for _, obj := range []struct {
		Remaining string
		Hostname  string
	}{
		{"root@HA-备 #", "HA-备"},
		{"[root@localhost ~]#", "localhost"},
		{"[root@localhost.localdomain ~]$", "localhost.localdomain"},
		{"hostname#", "hostname"},
		{"<HUAWEI>hrp enable", "HUAWEI"},
		{"中文主机名 #", "中文主机名"},
		{"HRP_M[HUAWEI] diagnose", "HUAWEI"},
		{"S-ABC-D1-EFG-~(M)# ", "S-ABC-D1-EFG-"},
		{"[mon@m41205302.cloud.208.am49 /home/mon]", "m41205302.cloud.208.am49"},
	} {
		assert.Equal(t, obj.Hostname, findHostname(obj.Remaining))
	}
}

// newTestReadWriter 用 io.Pipe 模拟 shell 的标准输入输出
//
//	返回 ReadWriter、模拟 shell 的 stdin 读取端、模拟 shell 的 stdout 写入端
func newTestReadWriter(cfg Config) (*ReadWriter, io.ReadCloser, io.WriteCloser) {
	inReader, inWriter := io.Pipe()   // ReadWriter 的写入端
	outReader, outWriter := io.Pipe() // ReadWriter 的读取端
	return New(inWriter, outReader, nil, cfg), inReader, outWriter
}

// TestReadWriter_ReadToEndLine 端到端：模拟 shell 回显命令并输出提示符
func TestReadWriter_ReadToEndLine(t *testing.T) {
	rw, inReader, outWriter := newTestReadWriter(Config{
		ReadConfirmWait: 10 * time.Millisecond,
		ReadConfirm:     2,
	})
	defer rw.Stop()

	// 模拟 shell：回显收到的命令，并输出提示符
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := inReader.Read(buf)
			if err != nil {
				return
			}
			_, _ = outWriter.Write([]byte("echo:" + string(buf[:n]) + "prompt# "))
		}
	}()

	assert.NoError(t, rw.Write("ls -l"))
	var lines []string
	assert.NoError(t, rw.ReadToEndLine(5*time.Second, func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, misc.HasLine(lines, "echo:ls -l"))
	assert.Equal(t, "prompt# ", rw.Prompt())
}

// TestReadWriter_Prompt_Concurrent 读取过程中并发调用Prompt不应出现数据竞争（go test -race 下运行）
func TestReadWriter_Prompt_Concurrent(t *testing.T) {
	rw, inReader, outWriter := newTestReadWriter(Config{
		ReadConfirmWait: 10 * time.Millisecond,
		ReadConfirm:     2,
	})
	defer rw.Stop()

	go func() {
		buf := make([]byte, 1024)
		for {
			if _, err := inReader.Read(buf); err != nil {
				return
			}
			_, _ = outWriter.Write([]byte("echo:cmd\nprompt# "))
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			_ = rw.Prompt()
			_ = rw.IsEndLine("prompt#")
		}
	}()

	assert.NoError(t, rw.Write("ls"))
	var lines []string
	assert.NoError(t, rw.ReadToEndLine(5*time.Second, func(arr []string) {
		lines = append(lines, arr...)
	}))
	<-done
	assert.True(t, misc.HasLine(lines, "echo:cmd"))
	assert.Equal(t, "prompt# ", rw.Prompt())
}

// TestReadWriter_WriteRaw_AfterStop Stop之后调用WriteRaw应返回错误而不是panic
func TestReadWriter_WriteRaw_AfterStop(t *testing.T) {
	rw, _, _ := newTestReadWriter(Config{})
	rw.Stop()
	assert.Error(t, rw.WriteRaw([]byte("test")))
	assert.Error(t, rw.Write("test"))
}
