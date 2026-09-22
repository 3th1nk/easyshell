package easyshell

// trimEmptyLines 移除前后的空行
func trimEmptyLines(a []string) []string {
	start := 0
	for ; start < len(a); start++ {
		if a[start] != "" {
			break
		}
	}
	a = a[start:]

	end := len(a)
	for end > 0 && a[end-1] == "" {
		end--
	}
	return a[:end]
}
