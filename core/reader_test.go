package core

import (
	"context"
	"errors"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDefaultPromptRegex(t *testing.T) {
	var r Reader
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
		if obj.Expect != r.IsPrompt(obj.Prompt) {
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

// newTestReader 用 io.Pipe 模拟 shell 的标准输入输出
//
//	返回 Reader、模拟 shell 的 stdin 读取端、模拟 shell 的 stdout 写入端
func newTestReader(cfg Config) (*Reader, io.ReadCloser, io.WriteCloser) {
	inReader, inWriter := io.Pipe()   // Reader 的写入端
	outReader, outWriter := io.Pipe() // Reader 的读取端
	return NewReader(inWriter, outReader, nil, cfg), inReader, outWriter
}

// mockShell 模拟 shell：收到命令后回显输出并给出提示符
func mockShell(inReader io.ReadCloser, outWriter io.WriteCloser, respond func(cmd string) string) {
	buf := make([]byte, 4096)
	for {
		n, err := inReader.Read(buf)
		if err != nil {
			return
		}
		_, _ = outWriter.Write([]byte(respond(strings.TrimSpace(string(buf[:n])))))
	}
}

func defaultTestConfig() Config {
	return Config{ReadConfirmWait: 10 * time.Millisecond, ReadConfirm: 2}
}

// TestReader_Run 验证 Run 便捷方法与提示符匹配
func TestReader_Run(t *testing.T) {
	r, inReader, outWriter := newTestReader(defaultTestConfig())
	defer r.Close()

	go mockShell(inReader, outWriter, func(cmd string) string {
		return "out:" + cmd + "\nprompt# "
	})

	var lines []string
	assert.NoError(t, r.Run(context.Background(), "show version", func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, hasLine(lines, "out:show version"))
	assert.Equal(t, "prompt# ", r.Prompt())
}

// TestReader_Run_Sequential 连续多次Run：上一条的隐藏提示符不能拼进下一条的输出
func TestReader_Run_Sequential(t *testing.T) {
	r, inReader, outWriter := newTestReader(defaultTestConfig())
	defer r.Close()

	go mockShell(inReader, outWriter, func(cmd string) string {
		return "out:" + cmd + "\nprompt# "
	})

	for i := 0; i < 3; i++ {
		var lines []string
		cmd := "cmd" + string(rune('1'+i))
		assert.NoError(t, r.Run(context.Background(), cmd, func(arr []string) {
			lines = append(lines, arr...)
		}))
		assert.Equal(t, []string{"out:" + cmd}, lines, "第%d次Run", i+1)
	}
}

// TestReader_ReadAll 验证读取全部输出直到流结束
func TestReader_ReadAll(t *testing.T) {
	r, inReader, outWriter := newTestReader(defaultTestConfig())
	defer r.Close()

	go func() {
		// 收到命令后输出内容并关闭输出(模拟命令执行结束)，ReadAll 依赖流结束
		buf := make([]byte, 64)
		if _, err := inReader.Read(buf); err == nil {
			_, _ = outWriter.Write([]byte("line1\nline2\nline3\n"))
		}
		_ = outWriter.Close()
	}()

	var lines []string
	assert.NoError(t, r.RunAll(context.Background(), "cat", func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, hasLine(lines, "line1"))
	assert.True(t, hasLine(lines, "line3"))
}

// TestReader_ConcurrentRead 并发 Read 应被立即拒绝
func TestReader_ConcurrentRead(t *testing.T) {
	r, _, outWriter := newTestReader(defaultTestConfig())
	defer r.Close()

	done := make(chan error, 1)
	go func() {
		done <- r.ReadUntilPrompt(context.Background(), func([]string) {})
	}()
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	err := r.ReadUntilPrompt(context.Background(), func([]string) {})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrConcurrentRead), "应返回并发读取错误, got=%v", err)
	assert.Less(t, time.Since(start), time.Second)

	_, _ = outWriter.Write([]byte("prompt# "))
	assert.NoError(t, <-done)
}

// TestReader_Close 后 Write/Read 返回 ErrClosed，Close 幂等
func TestReader_Close(t *testing.T) {
	r, _, _ := newTestReader(defaultTestConfig())
	assert.NoError(t, r.Close())
	assert.NoError(t, r.Close())

	err := r.Write("cmd")
	assert.Error(t, err)
	assert.True(t, IsClosed(err))

	err = r.ReadUntilPrompt(context.Background(), func([]string) {})
	assert.Error(t, err)
	assert.True(t, IsClosed(err))
}

// TestReader_Prompt_Concurrent 读取过程中并发查询 Prompt/IsPrompt 不应出现数据竞争
func TestReader_Prompt_Concurrent(t *testing.T) {
	r, inReader, outWriter := newTestReader(defaultTestConfig())
	defer r.Close()

	go mockShell(inReader, outWriter, func(string) string {
		return "echo:cmd\nprompt# "
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = r.Prompt()
			_ = r.IsPrompt("prompt#")
		}
	}()

	assert.NoError(t, r.Write("ls"))
	assert.NoError(t, r.ReadUntilPrompt(context.Background(), func([]string) {}))
	wg.Wait()
	assert.Equal(t, "prompt# ", r.Prompt())
}

// TestReader_Interceptor 验证拦截器自动应答(More 翻页场景)
func TestReader_Interceptor(t *testing.T) {
	r, inReader, outWriter := newTestReader(defaultTestConfig())
	defer r.Close()

	// 模拟设备：每次收到空格应答后输出后续页，共3页后输出提示符
	pages := 0
	go func() {
		buf := make([]byte, 64)
		first := true
		for {
			if _, err := inReader.Read(buf); err != nil {
				return
			}
			if first {
				first = false
			} else {
				pages++
			}
			switch pages {
			case 0:
				_, _ = outWriter.Write([]byte("page1\n---- More ----"))
			case 1:
				_, _ = outWriter.Write([]byte("page2\n---- More ----"))
			case 2:
				_, _ = outWriter.Write([]byte("page3\nprompt# "))
			}
		}
	}()

	var lines []string
	assert.NoError(t, r.Run(context.Background(), "display all", func(arr []string) {
		lines = append(lines, arr...)
	}, interceptor.More()))
	assert.Equal(t, 2, pages, "应自动应答2次翻页")
	t.Logf("lines=%q", lines)
	assert.True(t, hasLine(lines, "page1"))
	assert.True(t, hasLine(lines, "page3"))
	assert.False(t, hasLine(lines, "More"), "More提示不应出现在输出中")
}

// TestReader_Stderr 验证 stderr 三种策略
func TestReader_Stderr(t *testing.T) {
	for _, obj := range []struct {
		name   string
		policy StderrPolicy
		err    bool // 是否期望返回错误
		out    bool // stderr 是否进入输出
	}{
		{"error", StderrError, true, false},
		{"output", StderrOutput, false, true},
		{"ignore", StderrIgnore, false, false},
	} {
		t.Run(obj.name, func(t *testing.T) {
			cfg := defaultTestConfig()
			cfg.Stderr = obj.policy
			outReader, outWriter := io.Pipe()
			errReader, errWriter := io.Pipe()
			r := NewReader(io.Discard, outReader, errReader, cfg)
			defer r.Close()

			_, _ = errWriter.Write([]byte("some error\n"))
			_ = errWriter.Close()
			_ = outWriter.Close() // stdout 结束后进入 stderr 收尾阶段

			var lines []string
			err := r.ReadAll(context.Background(), func(arr []string) {
				lines = append(lines, arr...)
			})
			if obj.err {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "some error")
			} else {
				assert.NoError(t, err)
			}
			if obj.out {
				assert.True(t, hasLine(lines, "some error"))
			}
		})
	}
}

// TestReader_MultiByte 跨 chunk 的多字节字符不被截断(GBK 解码场景)
func TestReader_MultiByte(t *testing.T) {
	cfg := Config{
		ReadConfirmWait: 10 * time.Millisecond,
		ReadConfirm:     2,
		Decoder: func(b []byte) ([]byte, error) {
			return gbkToUTF8(b), nil
		},
	}
	r, _, outWriter := newTestReader(cfg)
	defer r.Close()

	// GBK 编码的 "中文提示" + \n，逐字节喂入(模拟网络分包在最恶劣情况下切分)
	gbk := utf8ToGBK("中文内容")
	go func() {
		for _, b := range gbk {
			_, _ = outWriter.Write([]byte{b})
		}
		_, _ = outWriter.Write([]byte("\nprompt# "))
	}()

	var lines []string
	assert.NoError(t, r.ReadUntilPrompt(context.Background(), func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.Equal(t, []string{"中文内容"}, lines)
}

func hasLine(lines []string, find string) bool {
	for _, s := range lines {
		if strings.Contains(s, find) {
			return true
		}
	}
	return false
}

// gbkToUTF8 / utf8ToGBK 测试辅助：GBK 与 UTF-8 互转
func gbkToUTF8(b []byte) []byte {
	out, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err != nil {
		return b
	}
	return out
}

func utf8ToGBK(s string) []byte {
	out, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(s))
	if err != nil {
		return []byte(s)
	}
	return out
}

// TestReader_RunPrompt 验证命令级提示符覆盖(严格模式)：
//
//	输出中存在会被宽松默认规则误匹配的行(如 "Description:")时，
//	默认 Run 会提前误判结束；RunPrompt 指定提示符后严格匹配，直到真正的新提示符
func TestReader_RunPrompt(t *testing.T) {
	r, inReader, outWriter := newTestReader(defaultTestConfig())
	defer r.Close()

	// 模拟设备：执行 "conf t" 后提示符从 <SW01> 变为 (config)#，且配置输出里有以冒号结尾的行
	go func() {
		buf := make([]byte, 4096)
		inConfig := false
		for {
			n, err := inReader.Read(buf)
			if err != nil {
				return
			}
			cmd := strings.TrimSpace(string(buf[:n]))
			switch {
			case cmd == "conf t":
				inConfig = true
				_, _ = outWriter.Write([]byte("Enter configuration commands\n(config)# "))
			case inConfig && cmd == "exit":
				inConfig = false
				_, _ = outWriter.Write([]byte("<SW01>"))
			case inConfig:
				_, _ = outWriter.Write([]byte("Description: this line would fool loose rule\n(config)# "))
			default:
				_, _ = outWriter.Write([]byte("out:" + cmd + "\n<SW01>"))
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 进入配置模式：指定新提示符(严格匹配)
	assert.NoError(t, r.RunPrompt(ctx, "conf t", regexp.MustCompile(`\(config\)#\s*$`), nil))
	assert.True(t, r.InConfigMode(), "应识别为配置模式")

	// 配置模式下的输出含 "Description:" 行——宽松规则会误匹配，严格指定提示符则不会
	var lines []string
	assert.NoError(t, r.RunPrompt(ctx, "description test", regexp.MustCompile(`\(config\)#\s*$`), func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, hasLine(lines, "Description: this line would fool loose rule"))

	// 退出配置模式
	assert.NoError(t, r.RunPrompt(ctx, "exit", regexp.MustCompile(`<SW01>\s*$`), nil))
	assert.False(t, r.InConfigMode())
}

// TestReader_InConfigMode 启发式的边界用例
func TestReader_InConfigMode(t *testing.T) {
	for _, obj := range []struct {
		prompt string
		want   bool
	}{
		{"[SW03]", true},                // H3C/华为系统视图
		{"[SW03-vlan1]", true},          // 子视图
		{"SW01(config)#", true},         // Cisco 配置模式
		{"SW01(config-if)#", true},      // Cisco 子接口
		{"[HRP_M[SW03]diagnose]", true}, // 华为主备嵌套
		{"<SW03>", false},               // H3C/华为用户视图
		{"SW01#", false},                // Cisco 特权模式
		{"[root@test-01 ~]#", false},    // Linux(含@与~)
		{"[mon@host /home/mon]", false}, // Linux(含@与空格)
		{"root@test-01 $", false},       // Linux
		{"", false},
	} {
		var r Reader
		r.mu.Lock()
		r.prompt = obj.prompt
		r.mu.Unlock()
		if got := r.InConfigMode(); got != obj.want {
			t.Errorf("prompt=%q want=%v got=%v", obj.prompt, obj.want, got)
		}
	}
}
