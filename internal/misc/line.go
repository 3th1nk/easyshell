package misc

import "strings"

func HasLine(out []string, find ...string) bool {
	return limitLineCount(1, out, find...) != 0
}

func LineCount(out []string, find ...string) (n int) {
	return limitLineCount(len(out), out, find...)
}

func limitLineCount(limit int, out []string, find ...string) (n int) {
loop:
	for _, s := range out {
		for _, f := range find {
			if !strings.Contains(s, f) {
				continue loop
			}
		}
		if n = n + 1; n >= limit {
			return
		}
	}
	return
}

func Contains(out []string, find ...string) bool {
loop:
	for _, s := range out {
		for _, f := range find {
			if !strings.Contains(s, f) {
				continue loop
			}
		}
		return true
	}
	return false
}

// TrimEmptyLine 移除前后的空行
func TrimEmptyLine(a []string) []string {
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
