package filter

type Filter func(s []byte) []byte

// CtrlFilter 控制字符过滤器
func CtrlFilter(s []byte) []byte {
	length := len(s)
	if length == 0 {
		return s
	}

	var drop [][2]int // 要丢弃的字符: []{起始位置(包含)，结束位置(不包含)}
	var pos int       // 最后一个字符的位置
	var backspace int // 连续的 \b 个数

	dropBackspace := func() {
		start := pos - backspace<<1
		if start < 0 {
			start = 0
		}
		drop = append(drop, [2]int{start, pos})
		backspace = 0
	}

loop:
	for pos < length {
		char := s[pos]
		if char == '\b' {
			backspace++
			pos++
			continue
		}
		if backspace != 0 {
			dropBackspace()
		}

		// 控制字符，这里仅处理了部分CSI控制符
		//	参考资料：
		//	https://vt100.net/docs/vt100-ug/chapter3.html#S3.3
		//	https://en.wikipedia.org/wiki/ANSI_escape_code#CSI_(Control_Sequence_Introducer)_sequences
		// TODO 对于修改内容的控制符，应该同时处理对应的内容
		// 	由于当前函数是在每次读操作之后调用，涉及修改的内容可能还未被完整的读取到缓存区，从而导致错误的处理，目前暂不处理
		if char == '\x1b' {
			if i := pos + 1; i < length && s[i] == '[' {
				n1 := i + 1
				if n1 < length {
					switch s[n1] {
					case
						's', // \x1b[s	保存光标位置
						'u', // \x1b[u	恢复光标位置
						'J', // \x1b[J  清除光标右下屏全部字符
						'K': // \x1b[K  清除从光标到行尾的内容
						drop = append(drop, [2]int{pos, n1 + 1})
						pos = n1 + 1
						continue
					}
				}

				n2 := i + 2
				if n2 < length {
					switch string(s[n1 : n2+1]) {
					case
						"2h", // \x1b[2h	锁定键盘
						"2l", // \x1b[2l	解锁键盘
						"4i", // \x1b[4i	关闭辅助端口
						"5i", // \x1b[5i	打开辅助端口
						"6n": // \x1b[6n	设备状态报告
						drop = append(drop, [2]int{pos, n2 + 1})
						pos = n2 + 1
						continue
					}
				}

				n4 := i + 4
				if n4 < length {
					switch string(s[n1 : n4+1]) {
					case
						"?25l", // \x1b[?25l   隐藏光标
						"?25h": // \x1b[?25h   显示光标
						drop = append(drop, [2]int{pos, n4 + 1})
						pos = n4 + 1
						continue
					}
				}

				for n := n1; n < length; n++ {
					switch s[n] {
					case
						'f', // \x1b[n;mf	设置水平垂直位置
						'H', // \x1b[n;mH	设置光标位置
						'R': // \x1b[n;mR	报告光标位置
						matched := true
						for _, ch := range s[n1:n] {
							if ch == ';' || ('0' <= ch && ch <= '9') {
								continue
							}
							matched = false
							break
						}
						if matched {
							drop = append(drop, [2]int{pos, n + 1})
							pos = n + 1
						}
						continue loop

					case
						'@', // \x1b[n@     在光标处插入n个字符
						'A', // \x1b[nA    	光标上移n行
						'B', // \x1b[nB    	光标下移n行
						'C', // \x1b[nC    	光标右移n个字符
						'D', // \x1b[nD		光标左移n个字符
						'E', // \x1b[nE		将光标向下移到到n行的行首（非ANSI.SYS）
						'F', // \x1b[nF		将光标向上移到到n行的行首（非ANSI.SYS）
						'G', // \x1b[nG		将光标移动到当前行中的第n列（非ANSI.SYS）
						'J', // \x1b[nJ		擦除显示
						'K', // \x1b[nK		擦除行
						'L', // \x1b[nL    	在光标下插入n行
						'M', // \x1b[nM    	删除光标之下n行, 剩余行上移
						'P', // \x1b[nP	   	删除光标右边n个字符，剩余部分左移
						'S', // \x1b[nS		向上滚动n行（非ANSI.SYS）
						'T', // \x1b[nT		向下滚动n行（非ANSI.SYS）
						'X': // \x1b[nX	   	删除光标右边n个字符
						matched := true
						for _, ch := range s[n1:n] {
							if '0' <= ch && ch <= '9' {
								continue
							}
							matched = false
							break
						}
						if matched {
							drop = append(drop, [2]int{pos, n + 1})
							pos = n + 1
						}
						continue loop

					case 'm':
						// SGR参数，未处理ITU-T T.416，如 \x1b[ … 38:2:<Color-Space-ID>:<r>:<g>:<b>:<unused>:<CS tolerance>:<Color-Space: 0="CIELUV"; 1="CIELAB">m
						// 	\x1b[xm、\x1b[x;ym、\x1b[x;y;zm
						//	\x1b[x;y;z;<r>;<g>;<b>m
						//	\x1b[x:y:z:<r>:<g>:<b>m
						matched := true
						for _, ch := range s[n1:n] {
							if ch == ';' || ch == ':' || ('0' <= ch && ch <= '9') ||
								ch == '<' || ch == '>' || ch == 'r' || ch == 'g' || ch == 'b' {
								continue
							}
							matched = false
							break
						}
						if matched {
							drop = append(drop, [2]int{pos, n + 1})
							pos = n + 1
						}
						continue loop

					}
				}
			}
		}

		pos++
	}
	if backspace != 0 {
		dropBackspace()
	}

	return dropByte(s, drop)
}

// AbnormalFilter 异常字符过滤器
//	TODO to be improved
func AbnormalFilter(s []byte) []byte {
	length := len(s)
	if length == 0 {
		return s
	}

	var drop [][2]int
	for pos := 0; pos+1 < length; pos++ {
		if s[pos] == '\x00' && (s[pos+1] == '\x00' || s[pos+1] == '\x01') {
			drop = append(drop, [2]int{pos, pos + 1})
			continue
		}

		if s[pos] == '\xff' && (s[pos+1] == '\xfd' || s[pos+1] == '\xff') {
			drop = append(drop, [2]int{pos, pos + 1})
			continue
		}
	}

	return dropByte(s, drop)
}
