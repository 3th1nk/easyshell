package easyshell

import "strings"

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

// hasLine 判断是否包含所有关键词的行
func hasLine(lines []string, find ...string) bool {
	for _, s := range lines {
		matched := true
		for _, f := range find {
			if !strings.Contains(s, f) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
