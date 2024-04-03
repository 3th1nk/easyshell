package filter

type IFilter interface {
	// Do 过滤字符, 返回过滤后的字符, 不修改源数据
	Do(s []byte) []byte
}

// DefaultFilter 默认字符过滤器
type DefaultFilter struct {
	opt Options
}

type Options struct {
	Crlf        bool // 是否处理回车、换行
	CrTrimMode  int  // 回车字符剔除模式，仅在Crlf为true时有效，默认值 CrTrimModeBeginOfLine，部分设备可能会误删除内容，可设置为 CrTrimModeOnlyCr
	Backspace   bool // 是否处理退格
	AnsiEscape  bool // 是否处理ANSI转义字符
	Utf8Replace bool // 是否处理UTF8替换字符
}

func (o Options) IsNothingToDo() bool {
	return !o.Crlf && !o.Backspace && !o.AnsiEscape && !o.Utf8Replace
}

var DefaultOptions = Options{
	Crlf:        true,
	CrTrimMode:  CrTrimModeBeginOfLine,
	Backspace:   true,
	AnsiEscape:  true,
	Utf8Replace: true,
}

func NewDefaultFilter(opt ...Options) IFilter {
	if len(opt) > 0 {
		return DefaultFilter{opt: opt[0]}
	}
	return DefaultFilter{opt: DefaultOptions}
}

func (f DefaultFilter) Do(src []byte) []byte {
	if len(src) == 0 || f.opt.IsNothingToDo() {
		return src
	}

	// 为避免修改源数据，拷贝一份
	dst := make([]byte, len(src))
	copy(dst, src)

	if f.opt.Backspace {
		dst = backspaceFilter(dst)
	}
	if f.opt.Crlf {
		dst = crlfFilter(dst, f.opt.CrTrimMode)
	}

	// 其他字符
	var dropArr [][2]int
	var drop [2]int
	var found bool
	for pos := 0; pos < len(dst); {
		if f.opt.Utf8Replace {
			found, pos, drop = checkUTF8ReplaceChar(dst, pos)
			if found {
				dropArr = append(dropArr, drop)
				continue
			}
		}

		if f.opt.AnsiEscape {
			found, pos, drop = checkAnsiEscape(dst, pos)
			if found {
				dropArr = append(dropArr, drop)
				continue
			}
		}

		pos++
	}
	return dropMultiBytes(dst, dropArr)
}
