package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type antigravityRefreshRequest struct {
	ChannelID int `json:"channel_id"`
}

type antigravityOAuthCompleteRequest struct {
	Input string `json:"input"`
}

func antigravityOAuthSessionKey(channelID int, field string) string {
	return fmt.Sprintf("antigravity_oauth_%s_%d", field, channelID)
}

func parseAntigravityCallbackInput(input string) (code string, state string, err error) {
	v := strings.TrimSpace(input)
	if v == "" {
		return "", "", fmt.Errorf("empty input")
	}
	// Full URL pasted
	if strings.Contains(v, "code=") {
		u, parseErr := url.Parse(v)
		if parseErr == nil {
			q := u.Query()
			code = strings.TrimSpace(q.Get("code"))
			state = strings.TrimSpace(q.Get("state"))
			if code != "" {
				return code, state, nil
			}
		}
		q, parseErr := url.ParseQuery(v)
		if parseErr == nil {
			code = strings.TrimSpace(q.Get("code"))
			state = strings.TrimSpace(q.Get("state"))
			if code != "" {
				return code, state, nil
			}
		}
	}
	// Bare code
	return v, "", nil
}

// RefreshAntigravityChannelCredential 手动刷新 antigravity 渠道凭证
func RefreshAntigravityChannelCredential(c *gin.Context) {
	var req antigravityRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	if req.ChannelID <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid channel_id"})
		return
	}

	ch, err := model.GetChannelById(req.ChannelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if ch.Type != constant.ChannelTypeAntigravity {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Antigravity"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	newKey, _, refreshErr := service.RefreshAntigravityChannelCredential(ctx, req.ChannelID, service.AntigravityCredentialRefreshOptions{ResetCaches: true})
	if refreshErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("refresh failed: %v", refreshErr)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "credential refreshed",
		"project_id": newKey.ProjectID,
		"expiry":     newKey.Expiry,
	})
}

// StartAntigravityOAuth 启动 Antigravity Google OAuth 流程（不绑定渠道）
func StartAntigravityOAuth(c *gin.Context) {
	startAntigravityOAuthWithChannelID(c, 0)
}

// StartAntigravityOAuthForChannel 启动 Antigravity Google OAuth 流程（绑定渠道）
func StartAntigravityOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	startAntigravityOAuthWithChannelID(c, channelID)
}

func startAntigravityOAuthWithChannelID(c *gin.Context, channelID int) {
	if channelID > 0 {
		ch, err := model.GetChannelById(channelID, false)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if ch == nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
			return
		}
		if ch.Type != constant.ChannelTypeAntigravity {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Antigravity"})
			return
		}
	}

	flow, err := service.CreateAntigravityOAuthFlow()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	session := sessions.Default(c)
	session.Set(antigravityOAuthSessionKey(channelID, "state"), flow.State)
	session.Set(antigravityOAuthSessionKey(channelID, "created_at"), time.Now().Unix())
	_ = session.Save()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"authorize_url": flow.AuthorizeURL,
		},
	})
}

// CompleteAntigravityOAuth 完成 Antigravity Google OAuth（不绑定渠道）
func CompleteAntigravityOAuth(c *gin.Context) {
	completeAntigravityOAuthWithChannelID(c, 0)
}

// CompleteAntigravityOAuthForChannel 完成 Antigravity Google OAuth（绑定渠道）
func CompleteAntigravityOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	completeAntigravityOAuthWithChannelID(c, channelID)
}

func completeAntigravityOAuthWithChannelID(c *gin.Context, channelID int) {
	req := antigravityOAuthCompleteRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	code, inputState, err := parseAntigravityCallbackInput(req.Input)
	if err != nil || strings.TrimSpace(code) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析授权信息失败，请检查输入格式"})
		return
	}

	channelProxy := ""
	if channelID > 0 {
		ch, err := model.GetChannelById(channelID, false)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if ch == nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
			return
		}
		if ch.Type != constant.ChannelTypeAntigravity {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Antigravity"})
			return
		}
		channelProxy = ch.GetSetting().Proxy
	}

	session := sessions.Default(c)
	expectedState, _ := session.Get(antigravityOAuthSessionKey(channelID, "state")).(string)

	// state 为空时（用户直接粘贴 code）仍允许，但 state 不匹配则拒绝
	if strings.TrimSpace(expectedState) != "" && strings.TrimSpace(inputState) != "" && inputState != expectedState {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "state mismatch, possible CSRF attack"})
		return
	}
	if strings.TrimSpace(expectedState) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "oauth flow not started or session expired, please start OAuth first"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := service.CompleteAntigravityOAuthWithProxy(ctx, code, expectedState, inputState, channelProxy)
	if err != nil {
		common.SysError("antigravity oauth complete failed: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "授权失败：" + err.Error()})
		return
	}

	oauthKey := service.AntigravityOAuthKey{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ProjectID:    result.ProjectID,
	}
	if !result.ExpiresAt.IsZero() {
		oauthKey.Expiry = result.ExpiresAt.Format(time.RFC3339)
	}
	encoded, err := common.Marshal(oauthKey)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	session.Delete(antigravityOAuthSessionKey(channelID, "state"))
	session.Delete(antigravityOAuthSessionKey(channelID, "created_at"))
	_ = session.Save()

	if channelID > 0 {
		if err := model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("key", string(encoded)).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		model.InitChannelCache()
		service.ResetProxyClientCache()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "saved",
			"data": gin.H{
				"channel_id": channelID,
				"email":      result.Email,
				"project_id": result.ProjectID,
				"expires_at": oauthKey.Expiry,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "generated",
		"data": gin.H{
			"key":        string(encoded),
			"email":      result.Email,
			"project_id": result.ProjectID,
			"expires_at": oauthKey.Expiry,
		},
	})
}
