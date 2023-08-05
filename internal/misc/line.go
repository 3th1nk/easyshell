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
	for i, s := range a {
		if s != "" {
			start = i
			break
		}
	}
	if start != 0 {
		a = a[start:]
	}

	end := len(a)
	for i := end - 1; i >= 0; i-- {
		if a[i] == "" {
			end = i
		} else {
			break
		}
	}
	if end != len(a) {
		a = a[:end]
	}

	return a
}
