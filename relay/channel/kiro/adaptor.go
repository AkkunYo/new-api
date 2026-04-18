package kiro

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	common.SysLog("[Kiro] Adaptor initialized - v3.8 (robust token refresh)")
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("kiro channel: endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("kiro channel: endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("kiro channel: endpoint not supported")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("kiro channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("kiro channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	// Debug: log Claude request
	common.SysLog(fmt.Sprintf("[Kiro] Claude request: messageCount=%d", len(req.Messages)))
	for i, msg := range req.Messages {
		var contentLen int
		var contentType string
		if msg.IsStringContent() {
			contentType = "string"
			contentLen = len(msg.GetStringContent())
		} else {
			contentType = "array"
			content, err := msg.ParseContent()
			if err != nil {
				common.SysLog(fmt.Sprintf("[Kiro]   claude_msg[%d]: ParseContent error: %v", i, err))
			}
			contentLen = len(content)
			// Log content block types
			for j, block := range content {
				common.SysLog(fmt.Sprintf("[Kiro]     block[%d]: type=%s", j, block.Type))
			}
		}
		common.SysLog(fmt.Sprintf("[Kiro]   claude_msg[%d]: role=%s, type=%s, contentLen=%d", i, msg.Role, contentType, contentLen))
	}

	// Claude 格式转 OpenAI 格式
	oaiReq, err := service.ClaudeToOpenAIRequest(*req, info)
	if err != nil {
		return nil, err
	}

	// Debug: log converted OpenAI messages
	common.SysLog(fmt.Sprintf("[Kiro] After conversion: messageCount=%d", len(oaiReq.Messages)))
	for i, msg := range oaiReq.Messages {
		var contentLen int
		if str, ok := msg.Content.(string); ok {
			contentLen = len(str)
		} else if contentList, ok := msg.Content.([]interface{}); ok {
			contentLen = len(contentList)
		}
		common.SysLog(fmt.Sprintf("[Kiro]   oai_msg[%d]: role=%s, contentLen=%d", i, msg.Role, contentLen))
	}

	return a.ConvertOpenAIRequest(c, info, oaiReq)
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	// 转换为 Kiro 格式
	payload, err := TranslateOpenAIToKiro(request)
	if err != nil {
		return nil, err
	}

	// 从 context 获取 profileArn
	if profileArn, exists := c.Get("kiro_profile_arn"); exists {
		if arn, ok := profileArn.(string); ok && arn != "" {
			payload.ProfileArn = arn
		}
	}

	return payload, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("kiro channel: /v1/responses not supported yet")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	// Kiro 使用固定端点，实际 URL 在 DoRequest 中动态选择
	return info.ChannelBaseUrl, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	key := strings.TrimSpace(info.ApiKey)
	oauthKey, err := ParseOAuthKey(key)
	if err != nil {
		return fmt.Errorf("kiro channel: %w", err)
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	profileArn := strings.TrimSpace(oauthKey.ProfileArn)
	clientId := strings.TrimSpace(oauthKey.ClientID)

	// 检查存储的 access_token 是否有效（距过期 30 秒内视为过期）
	needRefresh := accessToken == ""
	if !needRefresh && oauthKey.ExpiresAt != "" {
		if expiredAt, parseErr := time.Parse(time.RFC3339, oauthKey.ExpiresAt); parseErr == nil {
			needRefresh = time.Until(expiredAt) < 30*time.Second
		}
	}

	if needRefresh {
		// 内联刷新（新渠道首次使用或后台任务尚未刷新时的兜底）
		if strings.TrimSpace(oauthKey.RefreshToken) == "" {
			return errors.New("kiro channel: access_token expired and refresh_token is missing, please re-authorize the channel")
		}
		refreshCtx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		newKey, _, refreshErr := service.RefreshKiroChannelCredential(refreshCtx, info.ChannelId, service.KiroCredentialRefreshOptions{ResetCaches: true})
		if refreshErr != nil {
			return fmt.Errorf("kiro channel: failed to refresh token: %w", refreshErr)
		}
		accessToken = strings.TrimSpace(newKey.AccessToken)
		profileArn = strings.TrimSpace(newKey.ProfileArn)
		clientId = strings.TrimSpace(newKey.ClientID)
	}

	if accessToken == "" {
		return errors.New("kiro channel: access_token is empty")
	}

	if profileArn != "" {
		c.Set("kiro_profile_arn", profileArn)
	}

	machineId := GenerateMachineId(profileArn, clientId)

	var currentEndpoint KiroEndpoint
	if ep, exists := c.Get("kiro_current_endpoint"); exists {
		if endpoint, ok := ep.(KiroEndpoint); ok {
			currentEndpoint = endpoint
		}
	}

	req.Set("Authorization", "Bearer "+accessToken)
	req.Set("Content-Type", "application/json")
	req.Set("Accept", "*/*")
	req.Set("User-Agent", fmt.Sprintf("aws-sdk-js/1.0.34 ua/2.1 os/darwin lang/js md/nodejs#20.0.0 api/codewhispererstreaming#1.0.34 m/E KiroIDE-0.11.63-%s", machineId))
	req.Set("x-amz-user-agent", fmt.Sprintf("aws-sdk-js/1.0.34 KiroIDE-0.11.63-%s", machineId))
	req.Set("Amz-Sdk-Invocation-Id", uuid.New().String())
	req.Set("Amz-Sdk-Request", "attempt=1; max=3")
	req.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Set("x-amzn-codewhisperer-optout", "true")
	req.Set("x-amzn-codewhisperer-machine-id", machineId)

	if currentEndpoint.AmzTarget != "" {
		req.Set("X-Amz-Target", currentEndpoint.AmzTarget)
	}

	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	// 读取请求体
	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, err
	}

	// 获取端点列表
	endpoints := GetSortedEndpoints("auto", true)

	// 尝试每个端点
	var lastErr error
	for i, endpoint := range endpoints {
		// 存储端点到 context
		c.Set("kiro_current_endpoint", endpoint)

		// 构建请求
		req, err := http.NewRequest("POST", endpoint.URL, bytes.NewReader(bodyBytes))
		if err != nil {
			lastErr = err
			continue
		}

		// 设置 headers
		headers := req.Header
		if err := a.SetupRequestHeader(c, &headers, info); err != nil {
			lastErr = err
			continue
		}

		// 设置 req.Host（AWS API 要求）
		if parsedURL, parseErr := url.Parse(endpoint.URL); parseErr == nil {
			req.Host = parsedURL.Host
		}

		// 发起请求
		resp, err := channel.DoRequest(c, req, info)
		if err == nil {
			if i > 0 {
				common.SysLog(fmt.Sprintf("[Kiro] Switched to endpoint: %s", endpoint.Name))
			}
			return resp, nil
		}

		lastErr = err

		// 检查是否重试
		if httpErr, ok := err.(*types.NewAPIError); ok {
			if !ShouldRetryOnError(httpErr.StatusCode) {
				return nil, err
			}
			if i < len(endpoints)-1 {
				common.SysLog(fmt.Sprintf("[Kiro] Endpoint %s failed (HTTP %d), trying %s", endpoint.Name, httpErr.StatusCode, endpoints[i+1].Name))
			}
		}
	}

	return nil, fmt.Errorf("all Kiro endpoints failed: %w", lastErr)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	// 设置最终响应格式
	if info.RelayFormat == types.RelayFormatClaude {
		info.FinalRequestRelayFormat = types.RelayFormatClaude
	}

	// Kiro API 总是返回 AWS Event Stream 格式
	if info.RelayMode == relayconstant.RelayModeChatCompletions || info.RelayFormat == types.RelayFormatClaude {
		if info.IsStream {
			return KiroStreamHandler(c, info, resp)
		} else {
			return KiroHandler(c, info, resp)
		}
	}

	return nil, types.NewError(errors.New("kiro channel: unsupported relay mode"), types.ErrorCodeInvalidRequest)
}
