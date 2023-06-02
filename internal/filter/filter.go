package filter

type Filter func(s []byte) []byte

// DefaultFilter 默认字符过滤器
func DefaultFilter(s []byte) []byte {
	if len(s) == 0 {
		return s
	}

	// 处理退格
	s = backspaceFilter(s)
	// 处理回车换行
	s = crlfFilter(s)

	// 要丢弃的字符: []{起始位置(包含)，结束位置(不包含)}
	var dropArr [][2]int
	var drop [2]int
	var found bool
	for pos := 0; pos < len(s); {
		found, pos, drop = checkUTF8ReplaceChar(s, pos)
		if found {
			dropArr = append(dropArr, drop)
			continue
		}

		found, pos, drop = checkAnsiEscape(s, pos)
		if found {
			dropArr = append(dropArr, drop)
			continue
		}

		pos++
	}

	return dropByte(s, dropArr)
}
