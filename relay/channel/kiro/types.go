package kiro

import "encoding/json"

// KiroPayload Kiro API 请求格式（参考 Kiro-Go）
type KiroPayload struct {
	ConversationState ConversationState `json:"conversationState"`
	ProfileArn        string            `json:"profileArn,omitempty"`
	InferenceConfig   *InferenceConfig  `json:"inferenceConfig,omitempty"`
}

type ConversationState struct {
	AgentContinuationId string                `json:"agentContinuationId,omitempty"`
	AgentTaskType       string                `json:"agentTaskType"`
	ChatTriggerType     string                `json:"chatTriggerType"`
	ConversationID      string                `json:"conversationId"`
	CurrentMessage      KiroCurrentMessage    `json:"currentMessage"`
	History             []KiroHistoryMessage  `json:"history,omitempty"`
}

type KiroCurrentMessage struct {
	UserInputMessage KiroUserInputMessage `json:"userInputMessage"`
}

type KiroUserInputMessage struct {
	Content                 string                   `json:"content"`
	ModelID                 string                   `json:"modelId"`
	Origin                  string                   `json:"origin"`
	Images                  []KiroImage              `json:"images,omitempty"`
	UserInputMessageContext *UserInputMessageContext `json:"userInputMessageContext,omitempty"`
}

type UserInputMessageContext struct {
	Tools       []KiroToolWrapper `json:"tools,omitempty"`
	ToolResults []KiroToolResult  `json:"toolResults,omitempty"`
}

type KiroToolWrapper struct {
	ToolSpecification KiroToolSpecification `json:"toolSpecification"`
}

type KiroToolSpecification struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	JSON interface{} `json:"json"`
}

type KiroToolResult struct {
	ToolUseID string              `json:"toolUseId"`
	Content   []KiroResultContent `json:"content"`
	Status    string              `json:"status"`
}

type KiroResultContent struct {
	Text string `json:"text"`
}

type KiroImage struct {
	Format string `json:"format"`
	Source struct {
		Bytes string `json:"bytes"`
	} `json:"source"`
}

type KiroHistoryMessage struct {
	UserInputMessage         *KiroUserInputMessage         `json:"userInputMessage,omitempty"`
	AssistantResponseMessage *KiroAssistantResponseMessage `json:"assistantResponseMessage,omitempty"`
}

type KiroAssistantResponseMessage struct {
	Content  string        `json:"content"`
	ToolUses []KiroToolUse `json:"toolUses,omitempty"`
}

type KiroToolUse struct {
	ToolUseID string                 `json:"toolUseId"`
	Name      string                 `json:"name"`
	Input     map[string]interface{} `json:"input"`
}

type InferenceConfig struct {
	MaxTokens   int     `json:"maxTokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"topP,omitempty"`
}

// KiroEvent AWS Event Stream 事件格式
type KiroEvent struct {
	Content                string          `json:"content,omitempty"`
	Name                   string          `json:"name,omitempty"`
	ToolUseId              string          `json:"toolUseId,omitempty"`
	Input                  json.RawMessage `json:"input,omitempty"`
	Stop                   bool            `json:"stop,omitempty"`
	ContextUsagePercentage float64         `json:"contextUsagePercentage,omitempty"`
	FollowupPrompt         string          `json:"followupPrompt,omitempty"`
}
