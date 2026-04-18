package kiro

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// KiroStreamHandler 处理流式响应
func KiroStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)
	helper.SetEventStreamHeaders(c)

	usage := dto.Usage{
		PromptTokens:     0,
		CompletionTokens: 0,
		TotalTokens:      0,
	}

	createdTime := common.GetTimestamp()
	responseId := fmt.Sprintf("chatcmpl-%s", common.GetUUID())
	var fullResponseText strings.Builder

	reader := bufio.NewReader(resp.Body)
	buffer := ""
	readBuffer := make([]byte, 4096)

	for {
		n, err := reader.Read(readBuffer)
		if n > 0 {
			chunk := string(readBuffer[:n])
			buffer += chunk
			events, remaining := parseAwsEventStreamBuffer(buffer)
			buffer = remaining

			for _, event := range events {
				if event.Stop {
					continue
				}
				if event.Content != "" {
					info.SetFirstResponseTime()
					fullResponseText.WriteString(event.Content)

					contentPtr := &event.Content
					choice := dto.ChatCompletionsStreamResponseChoice{
						Index: 0,
						Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
							Content: contentPtr,
							Role:    "assistant",
						},
					}

					response := dto.ChatCompletionsStreamResponse{
						Id:      responseId,
						Object:  "chat.completion.chunk",
						Created: createdTime,
						Model:   info.UpstreamModelName,
						Choices: []dto.ChatCompletionsStreamResponseChoice{choice},
					}

					jsonData, _ := common.Marshal(response)
					_ = openai.HandleStreamFormat(c, info, string(jsonData), false, false)
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}

	responseText := fullResponseText.String()
	if responseText != "" {
		usage.CompletionTokens = len(responseText) / 4 // 简化 token 计算
		usage.OutputTokens = usage.CompletionTokens
	}

	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.InputTokens = usage.PromptTokens
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	finishReason := "stop"
	finishChunk := dto.ChatCompletionsStreamResponse{
		Id:      responseId,
		Object:  "chat.completion.chunk",
		Created: createdTime,
		Model:   info.UpstreamModelName,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: &finishReason,
				Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
			},
		},
		Usage: &usage,
	}

	lastStreamData := ""
	if jsonData, _ := common.Marshal(finishChunk); jsonData != nil {
		lastStreamData = string(jsonData)
	}

	openai.HandleFinalResponse(c, info, lastStreamData, responseId, createdTime, info.UpstreamModelName, "", &usage, false)

	return &usage, nil
}

// KiroHandler 处理非流式响应
func KiroHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	bodyStr := string(body)
	events, _ := parseAwsEventStreamBuffer(bodyStr)

	var fullContent strings.Builder
	for _, event := range events {
		if event.Stop {
			continue
		}
		if event.Content != "" {
			fullContent.WriteString(event.Content)
		}
	}

	responseText := fullContent.String()
	finishReason := "stop"
	message := dto.Message{Role: "assistant", Content: responseText}
	textChoice := dto.OpenAITextResponseChoice{Index: 0, Message: message, FinishReason: finishReason}
	textResponse := dto.TextResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", common.GetUUID()),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Model:   info.UpstreamModelName,
		Choices: []dto.OpenAITextResponseChoice{textChoice},
	}

	usage := &dto.Usage{
		PromptTokens:     info.GetEstimatePromptTokens(),
		CompletionTokens: len(responseText) / 4,
		TotalTokens:      0,
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	textResponse.Usage = *usage

	if info.RelayFormat == types.RelayFormatClaude {
		claudeContent := []dto.ClaudeMediaMessage{
			{Type: "text", Text: &responseText},
		}
		claudeResp := dto.ClaudeResponse{
			Id:         textResponse.Id,
			Type:       "message",
			Role:       "assistant",
			Content:    claudeContent,
			Model:      textResponse.Model,
			StopReason: "end_turn",
			Usage:      &dto.ClaudeUsage{InputTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens},
		}
		c.JSON(http.StatusOK, claudeResp)
	} else {
		c.JSON(http.StatusOK, textResponse)
	}

	return usage, nil
}

// parseAwsEventStreamBuffer 解析 AWS Event Stream 二进制格式
func parseAwsEventStreamBuffer(buffer string) ([]KiroEvent, string) {
	data := []byte(buffer)
	events := make([]KiroEvent, 0)
	offset := 0

	for offset < len(data) {
		if offset+12 > len(data) {
			break
		}

		totalLen := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
		if totalLen < 16 || offset+totalLen > len(data) {
			offset++
			continue
		}

		headersLen := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
		payloadStart := offset + 12 + headersLen
		payloadEnd := offset + totalLen - 4

		if payloadStart > payloadEnd {
			offset += totalLen
			continue
		}

		payload := data[payloadStart:payloadEnd]
		if len(payload) > 0 {
			var event KiroEvent
			if err := common.Unmarshal(payload, &event); err == nil {
				if event.Content != "" || event.Name != "" || event.Stop {
					events = append(events, event)
				}
			}
		}

		offset += totalLen
	}

	remaining := ""
	if offset < len(data) {
		remaining = string(data[offset:])
	}
	return events, remaining
}
