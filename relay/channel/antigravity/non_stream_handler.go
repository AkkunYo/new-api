package antigravity

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func AntigravityNonStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, err *types.NewAPIError) {
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewError(readErr, types.ErrorCodeDoRequestFailed)
	}

	var wrapper AntigravityStreamResponse
	if jsonErr := common.Unmarshal(body, &wrapper); jsonErr != nil {
		return nil, types.NewError(jsonErr, types.ErrorCodeBadResponse)
	}
	var responseBody AntigravityResponseBody
	if wrapper.Response != nil {
		responseBody = *wrapper.Response
	} else {
		responseBody.Candidates = wrapper.Candidates
	}

	var totalUsage *dto.Usage
	var fullContent string
	finishReason := "stop"

	if responseBody.UsageMetadata != nil {
		totalUsage = antigravityUsageToOpenAI(responseBody.UsageMetadata)
	}
	if len(responseBody.Candidates) > 0 {
		candidate := responseBody.Candidates[0]
		if reason := antigravityFinishReason(candidate, false); reason != nil {
			finishReason = *reason
		}
		if candidate.Content != nil {
			fullContent = antigravityTextFromParts(candidate.Content.Parts, false)
		}
	}

	message := dto.Message{Role: "assistant", Content: fullContent}
	textChoice := dto.OpenAITextResponseChoice{Index: 0, Message: message, FinishReason: finishReason}
	textResponse := dto.TextResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", common.GetUUID()),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Model:   info.UpstreamModelName,
		Choices: []dto.OpenAITextResponseChoice{textChoice},
	}

	if totalUsage == nil {
		totalUsage = &dto.Usage{
			PromptTokens:     info.GetEstimatePromptTokens(),
			CompletionTokens: len(fullContent) / 4,
		}
		totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
	}
	textResponse.Usage = *totalUsage

	if info.RelayFormat == types.RelayFormatClaude {
		claudeContent := []dto.ClaudeMediaMessage{{Type: "text", Text: &fullContent}}
		claudeResp := dto.ClaudeResponse{
			Id:         textResponse.Id,
			Type:       "message",
			Role:       "assistant",
			Content:    claudeContent,
			Model:      textResponse.Model,
			StopReason: "end_turn",
			Usage:      &dto.ClaudeUsage{InputTokens: totalUsage.PromptTokens, OutputTokens: totalUsage.CompletionTokens},
		}
		c.JSON(http.StatusOK, claudeResp)
	} else {
		c.JSON(http.StatusOK, textResponse)
	}

	return totalUsage, nil
}
