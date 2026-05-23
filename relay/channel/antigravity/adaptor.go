package antigravity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("antigravity channel: endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("antigravity channel: endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("antigravity channel: endpoint not supported")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("antigravity channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("antigravity channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	oaiReq, err := service.ClaudeToOpenAIRequest(*req, info)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, oaiReq)
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	oauthKey, err := ParseOAuthKey(strings.TrimSpace(info.ApiKey))
	if err != nil {
		return nil, fmt.Errorf("antigravity channel: %w", err)
	}
	projectID := strings.TrimSpace(oauthKey.ProjectID)
	if projectID == "" {
		return nil, errors.New("antigravity channel: project_id is empty")
	}
	return TranslateOpenAIToAntigravity(request, projectID)
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("antigravity channel: /v1/responses not supported yet")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return info.ChannelBaseUrl, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	key := strings.TrimSpace(info.ApiKey)
	oauthKey, err := ParseOAuthKey(key)
	if err != nil {
		return fmt.Errorf("antigravity channel: %w", err)
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	projectID := strings.TrimSpace(oauthKey.ProjectID)

	if oauthKey.IsExpiringSoon() || accessToken == "" {
		if strings.TrimSpace(oauthKey.RefreshToken) == "" {
			if accessToken == "" {
				return errors.New("antigravity channel: access_token expired and refresh_token is missing, please re-authorize the channel")
			}
		} else {
			refreshCtx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
			defer cancel()
			newKey, _, refreshErr := service.RefreshAntigravityChannelCredential(refreshCtx, info.ChannelId, service.AntigravityCredentialRefreshOptions{ResetCaches: true})
			if refreshErr != nil {
				if accessToken == "" {
					return fmt.Errorf("antigravity channel: failed to refresh token: %w", refreshErr)
				}
				// refresh failed but token not yet expired, fall back to existing token
			} else {
				accessToken = strings.TrimSpace(newKey.AccessToken)
				projectID = strings.TrimSpace(newKey.ProjectID)
			}
		}
	}

	if accessToken == "" {
		return errors.New("antigravity channel: access_token is empty")
	}
	if projectID == "" {
		return errors.New("antigravity channel: project_id is empty")
	}

	req.Set("Authorization", "Bearer "+accessToken)
	req.Set("Content-Type", "application/json")
	req.Set("User-Agent", "antigravity/1.24.0 darwin/arm64")

	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	path := "/v1internal:generateContent"
	if info.IsStream {
		path = "/v1internal:streamGenerateContent?alt=sse"
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, info.ChannelBaseUrl+path, requestBody)
	if err != nil {
		return nil, err
	}
	req.Close = true
	if err = a.SetupRequestHeader(c, &req.Header, info); err != nil {
		return nil, err
	}
	client, err := newAntigravityHTTPClient(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed, types.ErrOptionWithHideErrMsg("upstream error: do request failed"))
	}
	if resp == nil {
		return nil, errors.New("resp is nil")
	}
	_ = req.Body.Close()
	_ = c.Request.Body.Close()
	return resp, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.IsStream {
		return AntigravityStreamHandler(c, info, resp)
	}
	return AntigravityNonStreamHandler(c, info, resp)
}
