package telnet

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/3th1nk/easyshell/core"
	"net"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// 提示符匹配可能遇到一些异常情况：
// 	1、提示符后回显提示内容，这个已在鉴权时强制关闭回显
// 	2、终端不停打印日志内容，导致超时错误(无法正确匹配提示符 或者 无法判定登录成功的状态)，比如登录日志里面包含“LOGIN:”，这种情况必须关闭终端打印
var (
	DefaultUserRegex   = regexp.MustCompile(`(?i).*(login|user(name)?):\s*$`)
	DefaultPassRegex   = regexp.MustCompile(`(?i).*pass(word)?:\s*$`)
	DefaultPromptRegex = regexp.MustCompile(`.*[` + core.DefaultPromptTailChars + `]\s*$`)
)

type Client struct {
	c          net.Conn
	r          *bufio.Reader
	cfg        *ClientConfig
	welcomeStr string // 登录后的欢迎信息
	promptStr  string // 登录后的提示符
}

type ClientConfig struct {
	Addr          string
	User          string
	Password      string
	UserRegex     *regexp.Regexp // 匹配用户名提示符的正则
	PassRegex     *regexp.Regexp // 匹配密码提示符的正则
	PromptRegex   *regexp.Regexp // 匹配提示符的正则
	Timeout       time.Duration  // 连接超时时间, 默认15秒
	UnixWriteMode bool           // 如果设置，Write 将任何 '\n' (LF) 转换为 '\r\n' (CR LF)
	Echo          bool           // 如果设置，将允许回显（取决于服务端是否支持），部分网络设备上无效（总是回显）
	SuppressGA    bool           // 如果设置，将抑制 "go ahead" 命令
	TLS           *tls.Config
}

func NewClient(cfg *ClientConfig) (*Client, error) {
	if cfg.UserRegex == nil {
		cfg.UserRegex = DefaultUserRegex
	}
	if cfg.PassRegex == nil {
		cfg.PassRegex = DefaultPassRegex
	}
	if cfg.PromptRegex == nil {
		cfg.PromptRegex = DefaultPromptRegex
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}

	var (
		c   net.Conn
		err error
	)
	if cfg.TLS != nil {
		c, err = tls.DialWithDialer(&net.Dialer{Timeout: cfg.Timeout}, "tcp", cfg.Addr, cfg.TLS)
	} else {
		c, err = net.DialTimeout("tcp", cfg.Addr, cfg.Timeout)
	}
	if err != nil {
		return nil, err
	}

	client := &Client{
		c:   c,
		r:   bufio.NewReaderSize(c, 256),
		cfg: cfg,
	}
	defer func() {
		if err != nil {
			_ = client.Close()
		}
	}()

	if err = client.doAuth(); err != nil {
		return nil, err
	}
	return client, err
}

func (this *Client) Close() error {
	return this.c.Close()
}

func (this *Client) LocalAddr() net.Addr {
	return this.c.LocalAddr()
}

func (this *Client) RemoteAddr() net.Addr {
	return this.c.RemoteAddr()
}

func (this *Client) SetDeadline(t time.Time) error {
	return this.c.SetDeadline(t)
}

func (this *Client) SetReadDeadline(t time.Time) error {
	return this.c.SetReadDeadline(t)
}

func (this *Client) SetWriteDeadline(t time.Time) error {
	return this.c.SetWriteDeadline(t)
}

// Read is for implement an io.Reader interface
func (this *Client) Read(buf []byte) (int, error) {
	var n int
	for n < len(buf) {
		b, retry, err := this.doReadByte()
		if err != nil {
			return n, err
		}
		if !retry {
			buf[n] = b
			n++
		}
		if n > 0 && this.r.Buffered() == 0 {
			return n, nil
		}
	}
	return n, nil
}

// Write is for implements an io.Writer interface.
func (this *Client) Write(buf []byte) (n int, err error) {
	var s strings.Builder
	if this.cfg.UnixWriteMode {
		s.Write([]byte{cmd_IAC, LF})
	} else {
		s.WriteByte(cmd_IAC)
	}

	for len(buf) > 0 {
		var k int
		i := bytes.IndexAny(buf, s.String())
		if i == -1 {
			k, err = this.c.Write(buf)
			n += k
			break
		} else {
			k, err = this.c.Write(buf[:i])
			n += k
			if err != nil {
				break
			}
		}

		switch buf[i] {
		case LF:
			k, err = this.c.Write([]byte{CR, LF})
		case cmd_IAC:
			k, err = this.c.Write([]byte{cmd_IAC, cmd_IAC})
		}
		n += k
		if err != nil {
			break
		}
		buf = buf[i+1:]
	}
	return n, err
}

func (this *Client) SetEcho(echo bool) error {
	this.cfg.Echo = echo
	if echo {
		return this.do(opt_ECHO)
	}
	return this.doNot(opt_ECHO)
}

func (this *Client) SetSuppressGA(suppressGA bool) error {
	this.cfg.SuppressGA = suppressGA
	if suppressGA {
		return this.do(opt_SGA)
	}
	return this.doNot(opt_SGA)
}

func (this *Client) do(option byte) error {
	_, err := this.c.Write([]byte{cmd_IAC, cmd_DO, option})
	return err
}

func (this *Client) doNot(option byte) error {
	_, err := this.c.Write([]byte{cmd_IAC, cmd_DONT, option})
	return err
}

func (this *Client) will(option byte) error {
	_, err := this.c.Write([]byte{cmd_IAC, cmd_WILL, option})
	return err
}

func (this *Client) willNot(option byte) error {
	_, err := this.c.Write([]byte{cmd_IAC, cmd_WONT, option})
	return err
}

func (this *Client) sub(option byte, data ...byte) error {
	var buf = make([]byte, 0, len(data)+5)
	buf = append(buf, cmd_IAC, cmd_SB, option)
	buf = append(buf, data...)
	buf = append(buf, cmd_IAC, cmd_SE)
	_, err := this.c.Write(buf)
	return err
}

func (this *Client) allow(cmd, option byte) error {
	switch cmd {
	default:
		return nil
	case cmd_DO:
		return this.will(option)
	case cmd_DONT:
		return this.willNot(option)
	case cmd_WILL:
		return this.do(option)
	case cmd_WONT:
		return this.doNot(option)
	}
}

func (this *Client) deny(cmd, option byte) error {
	switch cmd {
	default:
		return nil
	case cmd_DO, cmd_DONT:
		return this.willNot(option)
	case cmd_WILL, cmd_WONT:
		return this.doNot(option)
	}
}

func (this *Client) skipSub() error {
	for {
		b, err := this.r.ReadByte()
		if err != nil {
			return err
		}

		if b == cmd_IAC {
			if b, err = this.r.ReadByte(); err != nil {
				return err
			} else if b == cmd_SE {
				return nil
			}
		}
	}
}

func (this *Client) answer(cmd, option byte) error {
	switch option {
	case opt_ECHO:
		if cmd == cmd_DONT || cmd == cmd_WONT {
			this.cfg.Echo = false
		}
		//util.PrintTimeLn("answer[%v ECHO]: %v", cmd, util.IfString(this.cfg.Echo, "allow", "deny"))
		if this.cfg.Echo {
			return this.allow(cmd, option)
		}
		return this.deny(cmd, option)

	case opt_SGA:
		if cmd == cmd_DONT || cmd == cmd_WONT {
			this.cfg.SuppressGA = false
		}
		//util.PrintTimeLn("answer[%v SGA]: %v", cmd, util.IfString(this.cfg.SuppressGA, "allow", "deny"))
		if this.cfg.SuppressGA {
			return this.allow(cmd, option)
		}
		return this.deny(cmd, option)

	case opt_NAWS:
		if cmd == cmd_WILL || cmd == cmd_WONT {
			//util.PrintTimeLn("answer[%v NAWS]: dont", cmd)
			return this.doNot(option)
		}

		//util.PrintTimeLn("answer[%v NAWS]: will", cmd)
		if err := this.will(option); err != nil {
			return err
		}
		// Reply with max window size: 65535x65535
		return this.sub(option, 255, 255, 255, 255)

	default:
		// Deny any other option
		//util.PrintTimeLn("answer[%v %v]: deny", cmd, option)
		return this.deny(cmd, option)
	}
}

// doReadByte 读取一个字节，如果遇到 IAC 命令，则处理之
func (this *Client) doReadByte() (b byte, retry bool, err error) {
	b, err = this.r.ReadByte()
	if nil != err || b != cmd_IAC {
		return b, false, err
	}

	if b, err = this.r.ReadByte(); nil != err {
		return b, false, err
	}
	switch b {
	default:
		return b, false, fmt.Errorf("unknown command: %d", b)

	case cmd_IAC:
		return b, false, nil

	case cmd_GA:
		return b, true, nil

	case cmd_WILL, cmd_WONT, cmd_DO, cmd_DONT:
		var option byte
		if option, err = this.r.ReadByte(); err == nil {
			err = this.answer(b, option)
		}
	case cmd_SB:
		err = this.skipSub()
	}
	if err != nil {
		return b, false, err
	}
	return b, true, nil
}

func (this *Client) ReadByte() (byte, error) {
	for {
		b, retry, err := this.doReadByte()
		if err != nil {
			return b, err
		}
		if !retry {
			return b, nil
		}
	}
}

func (this *Client) ReadRune() (r rune, size int, err error) {
loop:
	r, size, err = this.r.ReadRune()
	if err != nil {
		return
	}
	if r != unicode.ReplacementChar || size != 1 {
		return
	}

	if err = this.r.UnreadRune(); err != nil {
		return
	}
	// Read telnet command or escaped IAC
	_, retry, err := this.doReadByte()
	if err != nil {
		return
	}
	if retry {
		// This bad rune was a beginning of telnet command. Try read next rune.
		goto loop
	}
	// Return escaped IAC as unicode.ReplacementChar
	return
}

// doReadUtil 读取数据直到遇到指定的分隔符，返回读取的数据
func (this *Client) doReadUtil(read bool, delim byte) (bytes.Buffer, error) {
	var buf bytes.Buffer
	for {
		b, err := this.ReadByte()
		if err != nil {
			return buf, err
		}
		if read {
			buf.WriteByte(b)
		}
		if b == delim {
			return buf, nil
		}
	}
}

func (this *Client) ReadUtil(delim byte) (bytes.Buffer, error) {
	return this.doReadUtil(true, delim)
}

func (this *Client) SkipUtil(delim byte) error {
	_, err := this.doReadUtil(true, delim)
	return err
}

// doReadUtil2 读取数据直到遇到指定的分隔符之一，返回读取的数据和对应分隔符的索引
func (this *Client) doReadUtil2(read bool, delims ...string) (bytes.Buffer, int, error) {
	var buf bytes.Buffer
	if len(delims) == 0 {
		return buf, 0, fmt.Errorf("empty delims")
	}

	copyDelims := make([]string, len(delims))
	for i, delim := range delims {
		if delim == "" {
			return buf, 0, fmt.Errorf("empty delim[%d]", i)
		}
		copyDelims[i] = delim
	}

	for {
		b, err := this.ReadByte()
		if err != nil {
			return buf, 0, err
		}
		if read {
			buf.WriteByte(b)
		}

		for i, delim := range copyDelims {
			if delim[0] == b {
				if len(delim) == 1 {
					return buf, i, nil
				}
				copyDelims[i] = delim[1:]
			} else {
				copyDelims[i] = delims[i]
			}
		}
	}
}

// ReadUtil2 读取数据直到遇到指定的分隔符之一，返回读取的数据和对应分隔符的索引
func (this *Client) ReadUtil2(delims ...string) (bytes.Buffer, int, error) {
	return this.doReadUtil2(true, delims...)
}

// SkipUtil2 忽略数据直到遇到指定的分隔符之一，返回对应分隔符的索引
func (this *Client) SkipUtil2(delims ...string) (int, error) {
	_, i, err := this.doReadUtil2(false, delims...)
	return i, err
}

// doReadUtilPrompt 读取数据直到遇到提示符，返回读取的数据和提示符
//	该方法未处理 More、Continue 的情况
func (this *Client) doReadUtilPrompt() (data bytes.Buffer, prompt bytes.Buffer, err error) {
	for {
		var b byte
		b, err = this.ReadByte()
		if err != nil {
			return data, prompt, err
		}
		data.WriteByte(b)
		if this.cfg.PromptRegex.MatchString(data.String()) || this.cfg.UserRegex.MatchString(data.String()) || this.cfg.PassRegex.MatchString(data.String()) {
			if i := bytes.LastIndexByte(data.Bytes(), LF); i > 0 {
				prompt.WriteString(strings.TrimSpace(string(data.Bytes()[i+1:])))
				data.Truncate(i + 1)
			} else {
				prompt.WriteString(strings.TrimSpace(data.String()))
				data.Reset()
			}
			return data, prompt, nil
		}
	}
}

func (this *Client) Welcome() string {
	return this.welcomeStr
}

// FirstPrompt 获取登录后(跳过用户名、密码提示符)的第一个提示符
func (this *Client) FirstPrompt() string {
	return this.promptStr
}

// ScrollToNewLine 滚动到新的一行
func (this *Client) ScrollToNewLine() error {
	if _, err := this.Write([]byte{LF}); err != nil {
		return err
	}
	return this.SkipUtil(LF)
}

func (this *Client) doAuth() error {
	var firstRead = true
	var enterUser, enterPass bool
	for {
		// 读取数据直到遇到提示符
		data, prompt, err := this.doReadUtilPrompt()
		if err != nil {
			return err
		}
		// 首次读取的内容作为欢迎信息
		if firstRead {
			firstRead = false
			this.welcomeStr = strings.TrimSpace(data.String())
			strings.Replace(this.welcomeStr, "\r\n", "\n", -1)
		}

		// 如果没有指定用户名和密码，则直接返回；如果需要认证，则后续读写时会返回错误
		if this.cfg.User == "" || this.cfg.Password == "" {
			break
		}

		// 匹配到用户名提示符
		if this.cfg.UserRegex.MatchString(prompt.String()) {
			// 如果刚输入用户名、密码，再次读取到用户名提示符，说明用户名、密码错误
			if enterUser || enterPass {
				return fmt.Errorf("invalid username or password")
			}

			// 输入用户名
			if _, err = this.Write([]byte(this.cfg.User + "\n")); err != nil {
				return err
			}
			enterUser = true
			time.Sleep(time.Second)
			continue
		}

		// 匹配到密码提示符，输入密码
		if this.cfg.PassRegex.MatchString(prompt.String()) {
			// 如果刚输入密码，再次读取到密码提示符，说明密码错误
			if enterPass {
				return fmt.Errorf("invalid username or password")
			}

			if _, err = this.Write([]byte(this.cfg.Password + "\n")); err != nil {
				return err
			}
			enterPass = true
			time.Sleep(time.Second)
			continue
		}

		// 已输入过用户名或密码，且没有再次匹配到用户名或密码提示符，说明登录成功
		if enterUser || enterPass {
			this.promptStr = strings.TrimSpace(prompt.String())
			strings.Replace(this.promptStr, "\r\n", "\n", -1)
			break
		}
	}

	// 认证完成后切换到下一行，确保之后的读写是在新的一行(含提示符)
	_ = this.ScrollToNewLine()
	return nil
}
