package core

import (
	"context"
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
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

// TestReadWriter_ConcurrentRead 并发Read应被拒绝，避免互相争抢输出导致串流错乱
func TestReadWriter_ConcurrentRead(t *testing.T) {
	rw, _, outWriter := newTestReadWriter(Config{
		ReadConfirmWait: 10 * time.Millisecond,
		ReadConfirm:     2,
	})
	defer rw.Stop()

	// 第一个Read阻塞等待输出(不写数据)
	done := make(chan error, 1)
	go func() {
		done <- rw.ReadToEndLine(30*time.Second, func([]string) {})
	}()
	time.Sleep(100 * time.Millisecond)

	// 第二个Read应立即返回错误
	start := time.Now()
	err := rw.ReadToEndLine(30*time.Second, func([]string) {})
	assert.Error(t, err)
	assert.Less(t, time.Since(start), time.Second, "并发Read应立即返回错误")

	// 输出提示符让第一个Read正常结束
	_, _ = outWriter.Write([]byte("prompt# "))
	assert.NoError(t, <-done)
}

// TestReadWriter_Run 验证Run便捷方法(等价于Write+ReadToEndLine)
func TestReadWriter_Run(t *testing.T) {
	rw, inReader, outWriter := newTestReadWriter(Config{
		ReadConfirmWait: 10 * time.Millisecond,
		ReadConfirm:     2,
	})
	defer rw.Stop()

	// 模拟 shell：校验收到的命令，回显输出并给出提示符
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := inReader.Read(buf)
			if err != nil {
				return
			}
			cmd := strings.TrimSpace(string(buf[:n]))
			_, _ = outWriter.Write([]byte("out:" + cmd + "\nprompt# "))
		}
	}()

	var lines []string
	assert.NoError(t, rw.Run("show version", 5*time.Second, func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, misc.HasLine(lines, "out:show version"))
	assert.Equal(t, "prompt# ", rw.Prompt())

	// ctx 版本
	lines = nil
	assert.NoError(t, rw.RunContext(context.Background(), "display clock", func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, misc.HasLine(lines, "out:display clock"))

	// 已取消的context应返回canceled错误
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := rw.RunContext(ctx, "ls", func([]string) {})
	assert.Error(t, err)
	assert.True(t, IsCanceled(err))
}
