package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func Res2Str(obj interface{}) string {
	raw, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(raw)
}

func CutToSafeLength(content string, safeLen int) string {
	var (
		data  = []rune(content)
		total = len(data)
	)

	if total < safeLen {
		return content
	}

	buff := &bytes.Buffer{}
	buff.WriteString("The content has been truncated due to excessive length.\n")
	buff.WriteString(fmt.Sprintf("Total length: %d Returned length: %d\n", total, safeLen))
	buff.WriteString("\n")
	buff.WriteString(string(data[:safeLen]))
	return buff.String()
}

func GrepC(content string, C int, keywords ...string) string {
	keywordMap := make(map[string]struct{})
	for _, keyword := range keywords {
		keywordMap[strings.ToLower(keyword)] = struct{}{}
	}

	var (
		matches     []int
		resultLines = make(map[int]bool)
		buf         = &bytes.Buffer{}
	)

	contentLines := strings.Split(content, "\n")
	for i, line := range contentLines {
		line = strings.ToLower(line)
		for keyword := range keywordMap {
			if strings.Contains(line, keyword) {
				matches = append(matches, i)
			}
		}
	}

	for _, match := range matches {
		for i := match - C; i < match+C; i++ {
			resultLines[i] = true
		}
	}

	for i, line := range contentLines {
		if resultLines[i] {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}

	return buf.String()
}
