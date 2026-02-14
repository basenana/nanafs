package session

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/types"
)

var (
	// CompactThreshold is the token limit that triggers compaction.
	// Can be overridden via FRIDAY_COMPACT_THRESHOLD environment variable.
	CompactThreshold int64 = 128 * 1000 // 128K tokens default
)

func init() {
	if ctStr := os.Getenv("FRIDAY_COMPACT_THRESHOLD"); ctStr != "" {
		ct, _ := strconv.ParseInt(ctStr, 10, 64)
		if ct > 1000 {
			CompactThreshold = ct
		}
	}
}

func (s *Session) autoCompactHistory(ctx context.Context, req openai.Request) error {
	if s.compactThreshold < 0 {
		// disable compact
		return nil
	}

	beforeTokens := tokenCount(s.History)
	if beforeTokens < s.compactThreshold {
		return nil
	}

	err := s.CompactHistory(ctx)
	if err != nil {
		return err
	}

	req.SetHistory(s.History)
	return nil
}

func (s *Session) CompactHistory(ctx context.Context) error {
	// If no LLM, use simple truncation (keep last 40 messages)
	if s.llm == nil {
		s.mu.Lock()
		s.History = truncateToLastN(s.History, 40)
		s.mu.Unlock()
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	prompt := compactPrompt(s.History)
	req := openai.NewSimpleRequest("", types.Message{UserMessage: prompt})
	abstract, err := s.llm.CompletionNonStreaming(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	if abstract == "" {
		return fmt.Errorf("summary is empty")
	}

	s.History = RebuildHistoryWithAbstract(s.History, abstract)
	return nil
}

// truncateToLastN keeps only the last n messages.
func truncateToLastN(history []types.Message, n int) []types.Message {
	if len(history) <= n {
		return history
	}
	// Keep last n messages, preserving order
	return history[len(history)-n:]
}

// tokenCount calculates total tokens for a history.
func tokenCount(history []types.Message) int64 {
	var total int64
	for _, msg := range history {
		total += msg.FuzzyTokens()
	}
	return total
}

// compactPrompt generates the prompt for summarizing conversation history.
func compactPrompt(history []types.Message) string {
	return fmt.Sprintf(`Please summarize the following conversation history.
Focus on:
1. The main topics discussed
2. Key conclusions or decisions
3. Any important context that would be needed to continue the conversation

Your summary will be used to replace the old history, so make it comprehensive but concise.
This is the ONLY content you should output - do not add any introductions or conclusions.

CONVERSATION HISTORY:
%s

SUMMARY:`, formatHistoryForPrompt(history))
}

// formatHistoryForPrompt formats history for the summarization prompt.
func formatHistoryForPrompt(history []types.Message) string {
	var lines []string
	for i, msg := range history {
		switch {
		case msg.UserMessage != "":
			lines = append(lines, fmt.Sprintf("[User %d] %s", i+1, msg.UserMessage))
		case msg.AssistantMessage != "":
			lines = append(lines, fmt.Sprintf("[Assistant %d] %s", i+1, msg.AssistantMessage))
		case msg.AgentMessage != "":
			lines = append(lines, fmt.Sprintf("[Note %d] %s", i+1, msg.AgentMessage))
		case msg.ToolCallID != "":
			lines = append(lines, fmt.Sprintf("[Tool %d] %s(%s) -> %s",
				i+1, msg.ToolName, msg.ToolArguments, msg.ToolContent))
		}
	}
	return strings.Join(filterEmpty(lines, 3), "\n")
}

// filterEmpty filters out strings shorter than minLen.
func filterEmpty(lines []string, minLen int) []string {
	var result []string
	for _, line := range lines {
		if len(line) > minLen {
			result = append(result, line)
		}
	}
	return result
}

const summaryPrefix = `Several lengthy dialogues have already taken place. The following is a condensed summary of the progress of these historical dialogues:
`

// RebuildHistoryWithAbstract replaces old history with a summary.
func RebuildHistoryWithAbstract(history []types.Message, abstract string) []types.Message {
	if abstract == "" || len(history) == 0 {
		return history
	}

	effectiveHistory := make([]types.Message, 0, len(history))
	for _, msg := range history {
		if msg.UserMessage != "" || msg.AssistantMessage != "" || msg.AssistantReasoning != "" {
			effectiveHistory = append(effectiveHistory, msg)
		}
	}

	abstractMessage := types.Message{AgentMessage: summaryPrefix + abstract}
	if len(effectiveHistory) == 0 {
		return []types.Message{abstractMessage}
	}

	var (
		cutAt      = len(effectiveHistory) - 3
		newHistory []types.Message
	)

	if cutAt < 0 {
		cutAt = 0
	}

	if cutAt == 0 {
		// keep all and append abstracts
		effectiveHistory = append(effectiveHistory, abstractMessage)
		return effectiveHistory
	}

	keep := effectiveHistory[cutAt:]
	newHistory = append(newHistory, effectiveHistory[0])
	newHistory = append(newHistory, abstractMessage)
	newHistory = append(newHistory, keep...)
	return newHistory
}
