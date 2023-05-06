package filter

func checkBackspace(s []byte, pos int) (bool, int, [2]int) {
	// 连续的 \b 个数
	var backspace int

	length := len(s)
	for pos < length && s[pos] == '\b' {
		backspace++
		pos++
	}

	if backspace != 0 {
		start := pos - backspace<<1
		if start < 0 {
			start = 0
		}
		return true, pos, [2]int{start, pos}
	}

	return false, pos, [2]int{}
}
