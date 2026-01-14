package openai

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

type xmlParser struct {
	buf *xmlBuffer
}

func newXmlParser() *xmlParser {
	return &xmlParser{}
}

func (p *xmlParser) write(s string) []Delta {
	var (
		result  []Delta
		content string
	)
	for _, r := range s {
		if p.buf != nil {
			if msg, closed := p.buf.write(r); msg != nil || closed {
				if closed {
					p.buf = nil
				}
				if msg != nil {
					result = append(result, *msg)
				}
			}
			continue
		}

		if r == '<' {
			// before xml block content
			if content != "" {
				result = append(result, Delta{Content: content})
				content = ""
			}

			p.buf = &xmlBuffer{}
			p.buf.write('<')
			continue
		}

		content += string(r)
	}

	if content != "" {
		result = append(result, Delta{Content: content})
	}

	if len(result) > 5 {
		return compactMessages(result)
	}

	return result
}

func (p *xmlParser) flush() []Delta {
	if p.buf != nil {
		d := p.buf.flush()
		if d != nil {
			return []Delta{*d}
		}
	}
	return nil
}

type xmlBuffer struct {
	start   bool
	end     bool
	bracket int
	deep    int

	content  []rune
	thinking bool
}

func (l *xmlBuffer) write(r rune) (*Delta, bool) {

	l.content = append(l.content, r)

	// <k>v</k>
	switch r {
	case '<':
		l.bracket += 1
		l.start = true
	case '/':
		if l.start {
			l.end = true
		}
		l.start = false
	case '>':
		l.bracket -= 1
		l.start = false
		if l.end { // closing tag
			l.end = false
			l.deep -= 1
		} else {
			if l.bracket == 0 { // new tag
				l.deep += 1

				if l.deep == 1 && (string(l.content) == "<think>" || string(l.content) == "<thinking>") {
					l.thinking = true
					l.content = l.content[:0]
				}
			}
		}

	default:
		l.start = false
		if l.thinking && l.bracket == 0 {
			return &Delta{Reasoning: string(r)}, false
		}
	}

	// normal close
	if l.deep == 0 && l.bracket == 0 {
		return l.flush(), true
	}

	if l.deep < 0 {
		return l.flush(), true
	}
	return nil, false
}

func (l *xmlBuffer) flush() *Delta {
	if l.thinking {
		return nil
	}

	content := strings.TrimSpace(string(l.content))
	if content == "" {
		return nil
	}
	return xmlBodyToMessage(content)
}

func xmlBodyToMessage(body string) *Delta {
	switch {
	case strings.HasPrefix(body, "<ToolUse>"):
		body = strings.ReplaceAll(body, "<ToolUse>", "<tool_use>")
		body = strings.ReplaceAll(body, "</ToolUse>", "</tool_use>")
		fallthrough
	case strings.Contains(body, "<tool_use>"):
		use := ToolUse{}
		err := xml.Unmarshal([]byte(body), &use)
		if err != nil && (use.Name == "" || use.Arguments == "") {
			use.Error = fmt.Sprintf("The tool %s is used in an incorrect format; please try using the tool again", use.Name)
		} else {
			argBody := make(map[string]interface{})
			if err = json.Unmarshal([]byte(use.Arguments), &argBody); err != nil {
				use.Error = fmt.Sprintf("The arguments passed to the tool %s is not a valid JSON.", use.Name)
			}
		}

		return &Delta{ToolUse: []ToolUse{use}}
	case strings.Contains(body, "<thinking>"):
		body = strings.ReplaceAll(body, "<thinking>", "<think>")
		body = strings.ReplaceAll(body, "</thinking>", "</think>")
		fallthrough
	case strings.Contains(body, "<think>"):
		r := Reasoning{}
		err := xml.Unmarshal([]byte(body), &r)
		if err == nil && r.Content != "" {
			return &Delta{Reasoning: r.Content}
		}
		return &Delta{Reasoning: body}
	}
	return &Delta{Content: body}
}

func compactMessages(messages []Delta) []Delta {
	var (
		reasoning string
		content   string
		result    []Delta
	)

	for i, d := range messages {

		switch {
		case d.Content != "":

			if reasoning != "" {
				result = append(result, Delta{Reasoning: reasoning})
				reasoning = ""
			}

			content += d.Content

		case d.Reasoning != "":

			if content != "" {
				result = append(result, Delta{Content: content})
				content = ""
			}

			reasoning += d.Reasoning

		default:

			if reasoning != "" {
				result = append(result, Delta{Reasoning: reasoning})
				reasoning = ""
			}

			if content != "" {
				result = append(result, Delta{Content: content})
				content = ""
			}

			result = append(result, messages[i])

		}
	}

	if reasoning != "" {
		result = append(result, Delta{Reasoning: reasoning})
	}
	if content != "" {
		result = append(result, Delta{Content: content})
	}

	return result
}

func extractJSON(jsonContent string, model any) error {
	start := strings.Index(jsonContent, "{")
	if start == -1 {
		return fmt.Errorf("no JSON found")
	}

	jsonContent = jsonContent[start:]
	return json.NewDecoder(bytes.NewBuffer([]byte(jsonContent))).Decode(model)
}

func isTooManyError(err error) bool {
	return strings.Contains(err.Error(), "429 Too Many Requests")
}
