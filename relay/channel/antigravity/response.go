package antigravity

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

type AntigravityStreamResponse struct {
	Response   *AntigravityResponseBody `json:"response,omitempty"`
	Candidates []AntigravityCandidate   `json:"candidates,omitempty"`
	TraceID    string                   `json:"traceId,omitempty"`
}

type AntigravityResponseBody struct {
	ResponseID    string                 `json:"responseId,omitempty"`
	ModelVersion  string                 `json:"modelVersion,omitempty"`
	CreateTime    string                 `json:"createTime,omitempty"`
	Candidates    []AntigravityCandidate `json:"candidates,omitempty"`
	UsageMetadata *AntigravityUsage      `json:"usageMetadata,omitempty"`
}

type AntigravityCandidate struct {
	Content      *AntigravityContent `json:"content,omitempty"`
	FinishReason string              `json:"finishReason,omitempty"`
}

type AntigravityUsage struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount,omitempty"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}

func parseAntigravityStreamLine(line string) (*AntigravityResponseBody, error) {
	payload := strings.TrimSpace(line)
	payload = strings.TrimPrefix(payload, "data:")
	payload = strings.TrimSpace(payload)
	if payload == "" || payload == "[DONE]" {
		return nil, nil
	}

	var streamResp AntigravityStreamResponse
	if err := common.Unmarshal([]byte(payload), &streamResp); err != nil {
		return nil, err
	}
	if streamResp.Response != nil {
		return streamResp.Response, nil
	}
	if len(streamResp.Candidates) > 0 {
		return &AntigravityResponseBody{Candidates: streamResp.Candidates}, nil
	}
	return nil, nil
}

func antigravityUsageToOpenAI(usage *AntigravityUsage) *dto.Usage {
	if usage == nil {
		return nil
	}
	return &dto.Usage{
		PromptTokens:     usage.PromptTokenCount,
		CompletionTokens: usage.CandidatesTokenCount,
		TotalTokens:      usage.TotalTokenCount,
	}
}

func antigravityFinishReason(candidate AntigravityCandidate, sawToolCall bool) *string {
	finish := strings.ToUpper(strings.TrimSpace(candidate.FinishReason))
	if finish == "" {
		return nil
	}
	mapped := "stop"
	if sawToolCall {
		mapped = "tool_calls"
	} else if finish == "MAX_TOKENS" {
		mapped = "max_tokens"
	}
	return &mapped
}

func antigravityTextFromParts(parts []AntigravityPart, includeThoughts bool) string {
	var out strings.Builder
	for _, part := range parts {
		if part.Text == "" || (part.Thought && !includeThoughts) {
			continue
		}
		out.WriteString(part.Text)
	}
	return out.String()
}

func antigravityToolCallFromPart(part AntigravityPart, index int) *dto.ToolCallResponse {
	if part.FunctionCall == nil {
		return nil
	}
	args := "{}"
	if len(part.FunctionCall.Args) > 0 {
		if data, err := common.Marshal(part.FunctionCall.Args); err == nil {
			args = string(data)
		}
	}
	toolType := "function"
	call := &dto.ToolCallResponse{
		ID:   part.FunctionCall.ID,
		Type: toolType,
		Function: dto.FunctionResponse{
			Name:      part.FunctionCall.Name,
			Arguments: args,
		},
	}
	call.SetIndex(index)
	return call
}
