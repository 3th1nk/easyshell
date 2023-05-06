package filter

type Filter func(s []byte) []byte

// CtrlFilter 控制字符过滤器
func CtrlFilter(s []byte) []byte {
	length := len(s)
	if length == 0 {
		return s
	}

	// 要丢弃的字符: []{起始位置(包含)，结束位置(不包含)}
	var dropArr [][2]int

	for pos := 0; pos < length; {
		var found bool
		var drop [2]int
		found, pos, drop = checkBackspace(s, pos)
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
