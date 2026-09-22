package telnet

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"github.com/3th1nk/easyshell/v2/core"
	"net"
	"strings"
	"time"
)

// Client telnet 协议客户端，实现 io.ReadWriter。
//
//	登录认证在 NewClient 时完成；命令交互通过 Read/Write 与 core.Reader 配合完成。
type Client struct {
	c         net.Conn
	r         *bufio.Reader
	cfg       Config
	welcome   string // 登录后的欢迎信息
	promptStr string // 登录后的提示符
}

// NewClient 创建客户端并完成登录认证。
//
//	认证失败返回 core.Error{Op: OpAuth}，连接失败返回 core.Error{Op: OpDial}，
//	登录过程中的读写失败返回 core.Error{Op: OpRead}。
func NewClient(cfg Config) (*Client, error) {
	cfg = cfg.normalize()

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
		return nil, &core.Error{Op: core.OpDial, Addr: cfg.Addr, Err: err}
	}

	client := &Client{
		c:   c,
		r:   bufio.NewReaderSize(c, 4096),
		cfg: cfg,
	}
	defer func() {
		if err != nil {
			_ = client.Close()
		}
	}()

	if err = client.doAuth(); err != nil {
		// 认证失败返回auth错误，其余(超时、连接关闭等)返回read错误
		if core.IsAuth(err) {
			return nil, err
		}
		return nil, &core.Error{Op: core.OpRead, Addr: cfg.Addr, Err: err}
	}
	return client, nil
}

func (c *Client) Close() error {
	return c.c.Close()
}

func (c *Client) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

func (c *Client) RemoteAddr() net.Addr {
	return c.c.RemoteAddr()
}

func (c *Client) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

func (c *Client) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

func (c *Client) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}

// Read 实现 io.Reader：读取输出数据(跳过 telnet 协议命令字节)
func (c *Client) Read(buf []byte) (int, error) {
	var n int
	for n < len(buf) {
		b, retry, err := c.readByte()
		if err != nil {
			return n, err
		}
		if !retry {
			buf[n] = b
			n++
		}
		if n > 0 && c.r.Buffered() == 0 {
			return n, nil
		}
	}
	return n, nil
}

// Write 实现 io.Writer：写入数据，IAC 字节转义为双 IAC，UnixWriteMode 时 LF 转换为 CR LF。
//
//	n 的语义为实际写入网络的字节数(LF 转换为 CR LF 会计入 2 字节)。
//	注意：这里按字节查找转义目标，不能用 IndexAny 之类的字符集语义接口
//	(会把数据中的无效 UTF-8 字节误判为 IAC)。
func (c *Client) Write(buf []byte) (n int, err error) {
	_ = c.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
	defer func() {
		_ = c.SetWriteDeadline(time.Time{})
	}()

	for len(buf) > 0 {
		// 查找下一个需要转义处理的字节：IAC 转义为双 IAC，LF 在 UnixWriteMode 时转换为 CR LF
		i := bytes.IndexByte(buf, cmd_IAC)
		if c.cfg.UnixWriteMode {
			if j := bytes.IndexByte(buf, lf); j != -1 && (i == -1 || j < i) {
				i = j
			}
		}
		if i == -1 {
			var k int
			k, err = c.c.Write(buf)
			n += k
			return n, err
		}

		var k int
		k, err = c.c.Write(buf[:i])
		n += k
		if err != nil {
			return n, err
		}

		switch buf[i] {
		case lf:
			k, err = c.c.Write([]byte{cr, lf})
		case cmd_IAC:
			k, err = c.c.Write([]byte{cmd_IAC, cmd_IAC})
		}
		n += k
		if err != nil {
			return n, err
		}
		buf = buf[i+1:]
	}
	return n, nil
}

// SetEcho 设置是否允许回显
func (c *Client) SetEcho(echo bool) error {
	c.cfg.Echo = echo
	if echo {
		return c.do(opt_ECHO)
	}
	return c.doNot(opt_ECHO)
}

// SetSuppressGA 设置是否抑制 "go ahead" 命令
func (c *Client) SetSuppressGA(suppressGA bool) error {
	c.cfg.SuppressGA = suppressGA
	if suppressGA {
		return c.do(opt_SGA)
	}
	return c.doNot(opt_SGA)
}

func (c *Client) do(option byte) error {
	_, err := c.c.Write([]byte{cmd_IAC, cmd_DO, option})
	return err
}

func (c *Client) doNot(option byte) error {
	_, err := c.c.Write([]byte{cmd_IAC, cmd_DONT, option})
	return err
}

func (c *Client) will(option byte) error {
	_, err := c.c.Write([]byte{cmd_IAC, cmd_WILL, option})
	return err
}

func (c *Client) willNot(option byte) error {
	_, err := c.c.Write([]byte{cmd_IAC, cmd_WONT, option})
	return err
}

func (c *Client) sub(option byte, data ...byte) error {
	buf := make([]byte, 0, len(data)+5)
	buf = append(buf, cmd_IAC, cmd_SB, option)
	buf = append(buf, data...)
	buf = append(buf, cmd_IAC, cmd_SE)
	_, err := c.c.Write(buf)
	return err
}

func (c *Client) allow(cmd, option byte) error {
	switch cmd {
	default:
		return nil
	case cmd_DO:
		return c.will(option)
	case cmd_DONT:
		return c.willNot(option)
	case cmd_WILL:
		return c.do(option)
	case cmd_WONT:
		return c.doNot(option)
	}
}

func (c *Client) deny(cmd, option byte) error {
	switch cmd {
	default:
		return nil
	case cmd_DO, cmd_DONT:
		return c.willNot(option)
	case cmd_WILL, cmd_WONT:
		return c.doNot(option)
	}
}

// answer 应答服务端的协商请求
func (c *Client) answer(cmd, option byte) error {
	switch option {
	case opt_ECHO:
		if cmd == cmd_DONT || cmd == cmd_WONT {
			c.cfg.Echo = false
		}
		if c.cfg.Echo {
			return c.allow(cmd, option)
		}
		return c.deny(cmd, option)

	case opt_SGA:
		if cmd == cmd_DONT || cmd == cmd_WONT {
			c.cfg.SuppressGA = false
		}
		if c.cfg.SuppressGA {
			return c.allow(cmd, option)
		}
		return c.deny(cmd, option)

	case opt_NAWS:
		if cmd == cmd_WILL || cmd == cmd_WONT {
			return c.doNot(option)
		}
		if err := c.will(option); err != nil {
			return err
		}
		// Reply with max window size: 65535x65535(超大窗口可避免设备分页输出)
		return c.sub(option, 255, 255, 255, 255)

	default:
		// Deny any other option
		return c.deny(cmd, option)
	}
}

// readByte 读取一个字节，如果遇到 IAC 命令则处理之(retry=true 表示该字节属于协议命令，需继续读取)
func (c *Client) readByte() (b byte, retry bool, err error) {
	b, err = c.r.ReadByte()
	if err != nil || b != cmd_IAC {
		return b, false, err
	}

	if b, err = c.r.ReadByte(); err != nil {
		return b, false, err
	}
	switch b {
	default:
		return b, false, &core.Error{Op: core.OpRead, Addr: c.cfg.Addr, Err: errUnknownCommand(b)}

	case cmd_IAC:
		return b, false, nil

	case cmd_GA:
		return b, true, nil

	case cmd_WILL, cmd_WONT, cmd_DO, cmd_DONT:
		var option byte
		if option, err = c.r.ReadByte(); err == nil {
			err = c.answer(b, option)
		}
	case cmd_SB:
		err = c.skipSub()
	}
	if err != nil {
		return b, false, err
	}
	return b, true, nil
}

type unknownCommandError byte

func (e unknownCommandError) Error() string {
	return "telnet: unknown command code"
}

func errUnknownCommand(b byte) error {
	return unknownCommandError(b)
}

// readByteLoop 循环读取直到得到一个数据字节(跳过协议命令)
func (c *Client) readByteLoop() (byte, error) {
	for {
		b, retry, err := c.readByte()
		if err != nil {
			return b, err
		}
		if !retry {
			return b, nil
		}
	}
}

// skipSub 跳过子协商(SB...SE)的内容
func (c *Client) skipSub() error {
	for {
		b, err := c.r.ReadByte()
		if err != nil {
			return err
		}

		if b == cmd_IAC {
			if b, err = c.r.ReadByte(); err != nil {
				return err
			} else if b == cmd_SE {
				return nil
			}
		}
	}
}

// skipUntil 忽略数据直到遇到指定的分隔符
func (c *Client) skipUntil(delim byte) error {
	for {
		b, err := c.readByteLoop()
		if err != nil {
			return err
		}
		if b == delim {
			return nil
		}
	}
}

// readUntilPrompt 读取数据直到遇到提示符，返回读取的数据和提示符。
//
// 提示符匹配的已知限制：
//   - 该方法未处理 More、Continue 翻页场景(那是命令输出阶段的事)；
//   - 提示符后回显提示内容的问题已在鉴权时强制关闭回显；
//   - 终端不停打印日志会导致超时(无法正确匹配提示符)，比如登录日志里包含 "LOGIN:"，
//     这种情况必须先关闭终端日志打印。
func (c *Client) readUntilPrompt(timeout time.Duration) (data []byte, prompt []byte, err error) {
	if timeout > 0 {
		_ = c.SetReadDeadline(time.Now().Add(timeout))
		defer func() {
			_ = c.SetReadDeadline(time.Time{})
		}()
	}

	var buf bytes.Buffer
	for {
		var b byte
		b, err = c.readByteLoop()
		if err != nil {
			return buf.Bytes(), nil, err
		}
		buf.WriteByte(b)
		content := buf.Bytes()
		if c.cfg.LoginPromptRegex.Match(content) || c.cfg.LoginUserRegex.Match(content) || c.cfg.LoginPassRegex.Match(content) {
			if i := bytes.LastIndexByte(content, lf); i > 0 {
				prompt = append(prompt, strings.TrimSpace(string(content[i+1:]))...)
				buf.Truncate(i + 1)
			} else {
				prompt = append(prompt, strings.TrimSpace(buf.String())...)
				buf.Reset()
			}
			return buf.Bytes(), prompt, nil
		}
	}
}

// Welcome 返回登录后的欢迎信息
func (c *Client) Welcome() string {
	return c.welcome
}

// Prompt 返回登录后(跳过用户名、密码提示符)的第一个提示符
func (c *Client) Prompt() string {
	return c.promptStr
}

// scrollToNewLine 滚动到新的一行(发送 LF 并消费回显)
func (c *Client) scrollToNewLine() error {
	if _, err := c.Write([]byte{lf}); err != nil {
		return err
	}
	return c.skipUntil(lf)
}

func (c *Client) doAuth() error {
	var firstRead = true
	var enterUser, enterPass bool
	for {
		// 读取数据直到遇到提示符
		data, prompt, err := c.readUntilPrompt(c.cfg.Timeout)
		if err != nil {
			return err
		}
		// 首次读取的内容作为欢迎信息
		if firstRead {
			firstRead = false
			c.welcome = strings.ReplaceAll(strings.TrimSpace(string(data)), "\r\n", "\n")
		}

		// 如果没有指定用户名和密码，则直接返回；如果需要认证，则后续读写时会返回错误
		if c.cfg.User == "" && c.cfg.Password == "" {
			break
		}

		// 匹配到用户名提示符
		if c.cfg.LoginUserRegex.Match(prompt) {
			// 如果刚输入用户名、密码，再次读取到用户名提示符，说明用户名、密码错误
			if enterUser || enterPass {
				return &core.Error{Op: core.OpAuth, Addr: c.cfg.Addr, Err: errInvalidCredentials}
			}

			// 输入用户名
			if _, err = c.Write([]byte(c.cfg.User + "\n")); err != nil {
				return err
			}
			enterUser = true
			time.Sleep(time.Second)
			continue
		}

		// 匹配到密码提示符，输入密码
		if c.cfg.LoginPassRegex.Match(prompt) {
			// 如果刚输入密码，再次读取到密码提示符，说明密码错误
			if enterPass {
				return &core.Error{Op: core.OpAuth, Addr: c.cfg.Addr, Err: errInvalidCredentials}
			}

			if _, err = c.Write([]byte(c.cfg.Password + "\n")); err != nil {
				return err
			}
			enterPass = true
			time.Sleep(time.Second)
			continue
		}

		// 已输入过用户名或密码，且没有再次匹配到用户名或密码提示符，说明登录成功
		if enterUser || enterPass {
			c.promptStr = strings.ReplaceAll(strings.TrimSpace(string(prompt)), "\r\n", "\n")
			break
		}
	}

	// 认证完成后滚动到新的一行，确保之后的读写是在新的一行(含提示符)
	_ = c.scrollToNewLine()
	return nil
}

var errInvalidCredentials = errors.New("invalid username or password")
