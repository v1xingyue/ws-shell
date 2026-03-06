// Package envload 在 init 时从当前工作目录加载 .env 文件到环境变量。
// 需在 main 包中通过 import _ "wsterm/envload" 最先导入，以便在其它包级变量求值前完成加载。
package envload

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func init() {
	LoadFromCurrentDir()
}

// LoadFromCurrentDir 从当前工作目录加载 .env 文件。
// 若文件不存在或无法读取则静默忽略，不覆盖已存在的环境变量。
func LoadFromCurrentDir() {
	path := filepath.Join(".", ".env")
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	loadReader(f)
}

// loadReader 从 io.Reader 解析 KEY=VALUE 并设置到 os.Environ（不覆盖已存在）。
func loadReader(f *os.File) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if key == "" {
			continue
		}
		raw := strings.TrimSpace(line[idx+1:])
		value := unquoteEnvValue(raw)
		if _, set := os.LookupEnv(key); !set {
			_ = os.Setenv(key, value)
		}
	}
}

func unquoteEnvValue(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}
	if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
		inner := s[1 : len(s)-1]
		if s[0] == '"' {
			return unescapeDoubleQuoted(inner)
		}
		return inner
	}
	return trimTrailingComment(s)
}

func unescapeDoubleQuoted(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\', '"':
				b.WriteByte(s[i+1])
			default:
				b.WriteByte(s[i])
				b.WriteByte(s[i+1])
			}
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func trimTrailingComment(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || unicode.IsSpace(rune(s[i-1]))) {
			return strings.TrimSpace(s[:i])
		}
	}
	return s
}
