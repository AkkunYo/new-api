package antigravity

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const antigravityStreamScannerBuffer = 1024 * 1024 * 4

func AntigravityStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, err *types.NewAPIError) {
	defer resp.Body.Close()

	helper.SetEventStreamHeaders(c)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(nil, antigravityStreamScannerBuffer)
	scanner.Split(bufio.ScanLines)

	var totalUsage *dto.Usage
	createdTime := common.GetTimestamp()
	responseID := fmt.Sprintf("chatcmpl-%s", common.GetUUID())
	toolIndex := 0

	for scanner.Scan() {
		responseBody, parseErr := parseAntigravityStreamLine(scanner.Text())
		if parseErr != nil || responseBody == nil {
			continue
		}
		if responseBody.ResponseID != "" {
			responseID = responseBody.ResponseID
		}
		if responseBody.UsageMetadata != nil {
			totalUsage = antigravityUsageToOpenAI(responseBody.UsageMetadata)
		}
		if len(responseBody.Candidates) == 0 {
			continue
		}

		candidate := responseBody.Candidates[0]
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			continue
		}

		for _, part := range candidate.Content.Parts {
			choice := dto.ChatCompletionsStreamResponseChoice{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"},
			}
			if part.Text != "" {
				if part.Thought {
					choice.Delta.SetReasoningContent(part.Text)
				} else {
					choice.Delta.SetContentString(part.Text)
				}
			}
			if call := antigravityToolCallFromPart(part, toolIndex); call != nil {
				choice.Delta.ToolCalls = []dto.ToolCallResponse{*call}
				toolIndex++
			}
			if choice.Delta.Content == nil && choice.Delta.ReasoningContent == nil && len(choice.Delta.ToolCalls) == 0 {
				continue
			}

			info.SetFirstResponseTime()
			streamResponse := dto.ChatCompletionsStreamResponse{
				Id:      responseID,
				Object:  "chat.completion.chunk",
				Created: createdTime,
				Model:   info.UpstreamModelName,
				Choices: []dto.ChatCompletionsStreamResponseChoice{choice},
			}
			jsonData, _ := common.Marshal(streamResponse)
			_ = openai.HandleStreamFormat(c, info, string(jsonData), false, false)
		}

		if finishReason := antigravityFinishReason(candidate, toolIndex > 0); finishReason != nil {
			choice := dto.ChatCompletionsStreamResponseChoice{Index: 0, FinishReason: finishReason}
			streamResponse := dto.ChatCompletionsStreamResponse{
				Id:      responseID,
				Object:  "chat.completion.chunk",
				Created: createdTime,
				Model:   info.UpstreamModelName,
				Choices: []dto.ChatCompletionsStreamResponseChoice{choice},
				Usage:   totalUsage,
			}
			jsonData, _ := common.Marshal(streamResponse)
			_ = openai.HandleStreamFormat(c, info, string(jsonData), false, false)
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
	}
	if totalUsage == nil {
		totalUsage = &dto.Usage{}
	}
	helper.Done(c)
	return totalUsage, nil
}
