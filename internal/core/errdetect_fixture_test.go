package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDetectErrors_Fixtures 从 testdata/errdetect/<来源>/ 加载官方手册/真机样例：
//
//	hits.txt   每行(跳过空行与 # 注释)应命中错误检测
//	misses.txt 每行应不命中(日志/横幅/正常输出/业务错误等误报防护样例)
//
//	厂商手册或真机输出沉淀为新样例时，追加对应文件即可，无需改测试代码。
func TestDetectErrors_Fixtures(t *testing.T) {
	patterns := DefaultErrorPatterns()

	vendors, err := os.ReadDir("testdata/errdetect")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vendors {
		if !v.IsDir() {
			continue
		}
		for _, name := range []string{"hits.txt", "misses.txt"} {
			path := filepath.Join("testdata/errdetect", v.Name(), name)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				continue // 该目录未提供此类样例
			}
			lines := readFixtureLines(t, path)
			if len(lines) == 0 {
				t.Errorf("%s: 无有效样例", path)
				continue
			}
			for _, line := range lines {
				de := detectErrors(patterns, "fixture", []string{line})
				if name == "hits.txt" && de == nil {
					t.Errorf("%s: 应命中: %q", path, line)
				}
				if name == "misses.txt" && de != nil {
					t.Errorf("%s: 不应命中: %q -> %v", path, line, de)
				}
			}
		}
	}
}

func readFixtureLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
