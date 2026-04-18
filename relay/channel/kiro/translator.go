package kiro

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/google/uuid"
)

// TranslateOpenAIToKiro 将 OpenAI 格式转换为 Kiro 格式（参考 Kiro-Go）
func TranslateOpenAIToKiro(request *dto.GeneralOpenAIRequest) (*KiroPayload, error) {
	modelID := MapModel(request.Model)
	origin := "AI_EDITOR"

	// Debug: log incoming messages
	common.SysLog(fmt.Sprintf("[Kiro] TranslateOpenAIToKiro: messageCount=%d", len(request.Messages)))
	for i, msg := range request.Messages {
		content := extractMessageText(msg)
		common.SysLog(fmt.Sprintf("[Kiro]   msg[%d]: role=%s, contentLen=%d", i, msg.Role, len(content)))
	}

	// 提取 system prompt
	var systemPrompt string
	messages := request.Messages
	if len(messages) > 0 && messages[0].Role == "system" {
		systemPrompt = extractMessageText(messages[0])
		messages = messages[1:]
	}

	// 构建历史和当前消息
	history := make([]KiroHistoryMessage, 0)
	var currentContent string

	for i, msg := range messages {
		isLast := i == len(messages)-1

		if msg.Role == "user" {
			content := extractMessageText(msg)
			if isLast {
				currentContent = content
			} else {
				history = append(history, KiroHistoryMessage{
					UserInputMessage: &KiroUserInputMessage{
						Content: content,
						ModelID: modelID,
						Origin:  origin,
					},
				})
			}
		} else if msg.Role == "assistant" {
			content := extractMessageText(msg)
			history = append(history, KiroHistoryMessage{
				AssistantResponseMessage: &KiroAssistantResponseMessage{
					Content: content,
				},
			})
		}
	}

	// 构建最终内容（参考 Kiro-Go 格式）
	finalContent := ""
	if systemPrompt != "" {
		finalContent = "--- SYSTEM PROMPT ---\n" + systemPrompt + "\n--- END SYSTEM PROMPT ---\n\n"
	}
	if currentContent != "" {
		finalContent += currentContent
	} else {
		finalContent = "."
	}

	// 构建 payload
	payload := &KiroPayload{
		ConversationState: ConversationState{
			AgentContinuationId: uuid.New().String(),
			AgentTaskType:       "vibe",
			ChatTriggerType:     "MANUAL",
			ConversationID:      buildConversationID(modelID, systemPrompt, currentContent),
			CurrentMessage: KiroCurrentMessage{
				UserInputMessage: KiroUserInputMessage{
					Content: finalContent,
					ModelID: modelID,
					Origin:  origin,
				},
			},
			History: history,
		},
	}

	// InferenceConfig - 设置默认 maxTokens 为 4096
	maxTokens := 4096
	if request.MaxTokens != nil && int(*request.MaxTokens) > 0 {
		maxTokens = int(*request.MaxTokens)
	}

	payload.InferenceConfig = &InferenceConfig{
		MaxTokens: maxTokens,
	}

	if request.Temperature != nil {
		payload.InferenceConfig.Temperature = *request.Temperature
	}
	if request.TopP != nil {
		payload.InferenceConfig.TopP = *request.TopP
	}

	// Debug log
	maxTokensVal := "nil"
	if request.MaxTokens != nil {
		maxTokensVal = fmt.Sprintf("%d", int(*request.MaxTokens))
	}
	common.SysLog(fmt.Sprintf("[Kiro] Request: model=%s, maxTokens=%s, historyLen=%d, contentLen=%d, content=%s",
		modelID, maxTokensVal, len(history), len(finalContent), truncateString(finalContent, 100)))

	return payload, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// MapModel 映射模型名称
func MapModel(model string) string {
	modelMapping := map[string]string{
		"claude-sonnet-4-20250514":   "claude-sonnet-4.5",
		"claude-sonnet-4-5":          "claude-sonnet-4.5",
		"claude-sonnet-4.5":          "claude-sonnet-4.5",
		"claude-sonnet-4-6":          "claude-sonnet-4.6",
		"claude-sonnet-4.6":          "claude-sonnet-4.6",
		"claude-opus-4-7":            "claude-opus-4.7",
		"claude-opus-4.7":            "claude-opus-4.7",
		"claude-haiku-4-5":           "claude-haiku-4.5",
		"claude-haiku-4.5":           "claude-haiku-4.5",
		"claude-opus-4-5":            "claude-opus-4.5",
		"claude-opus-4.5":            "claude-opus-4.5",
		"claude-opus-4-6":            "claude-opus-4.6",
		"claude-opus-4.6":            "claude-opus-4.6",
		"claude-3-7-sonnet-20250219": "claude-3-7-sonnet-20250219",
	}

	lower := strings.ToLower(model)
	for key, value := range modelMapping {
		if strings.Contains(lower, key) {
			return value
		}
	}

	return "claude-sonnet-4.5"
}

// buildConversationID 生成会话 ID（参考 Kiro-Go）
func buildConversationID(modelID, systemPrompt, firstUserMessage string) string {
	anchor := modelID + "|" + systemPrompt + "|" + firstUserMessage
	hash := sha256.Sum256([]byte(anchor))
	return hex.EncodeToString(hash[:])
}

// extractMessageText 提取消息文本内容，使用 dto.Message.ParseContent() 处理所有类型
func extractMessageText(msg dto.Message) string {
	contents := msg.ParseContent()
	texts := make([]string, 0, len(contents))
	for _, item := range contents {
		if item.Type == "text" && item.Text != "" {
			texts = append(texts, item.Text)
		}
	}
	return strings.TrimSpace(strings.Join(texts, "\n"))
}
