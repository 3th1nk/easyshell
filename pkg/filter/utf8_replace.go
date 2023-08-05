package filter

// checkUTF8ReplaceChar 检查UTF8替换字符 0xEF 0XBF 0XBD
func checkUTF8ReplaceChar(s []byte, pos int) (bool, int, [2]int) {
	var cnt int
	for pos+2 < len(s) {
		if s[pos] == 0xEF && s[pos+1] == 0xBF && s[pos+2] == 0xBD {
			cnt++
			pos += 3
		} else {
			break
		}
	}

	if cnt > 0 {
		return true, pos, [2]int{pos - cnt*3, pos}
	}

	return false, pos, [2]int{}
}
