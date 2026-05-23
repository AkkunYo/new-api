package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/antigravity"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetAntigravityChannelUsage 获取 Antigravity 渠道的账号信息
func GetAntigravityChannelUsage(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelId, true)
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
	if ch.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	oauthKey, err := antigravity.ParseOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		common.SysError("failed to parse antigravity oauth key: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析凭证失败，请检查渠道配置"})
		return
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	if oauthKey.IsExpiringSoon() || accessToken == "" {
		if strings.TrimSpace(oauthKey.RefreshToken) == "" {
			if accessToken == "" {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "access_token 已过期且缺少 refresh_token，请重新授权渠道"})
				return
			}
			// token 快过期但无 refresh_token，降级用现有 token 尝试
		} else {
			refreshCtx, refreshCancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
			defer refreshCancel()
			newKey, _, refreshErr := service.RefreshAntigravityChannelCredential(refreshCtx, channelId, service.AntigravityCredentialRefreshOptions{ResetCaches: true})
			if refreshErr != nil {
				common.SysError("failed to refresh antigravity token for usage: " + refreshErr.Error())
				if accessToken == "" {
					c.JSON(http.StatusOK, gin.H{"success": false, "message": "刷新 token 失败，请检查渠道配置"})
					return
				}
				// 刷新失败但旧 token 未过期，降级继续
				common.SysLog("antigravity usage: refresh failed, falling back to existing token")
			} else {
				accessToken = strings.TrimSpace(newKey.AccessToken)
			}
		}
	}

	if accessToken == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "access_token 为空"})
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	var (
		accountStatusCode int
		accountBody       []byte
		accountErr        error
		modelsStatusCode  int
		modelsBody        []byte
		modelsErr         error
		wg                sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		accountStatusCode, accountBody, accountErr = service.FetchAntigravityAccountInfo(ctx, client, accessToken)
	}()
	go func() {
		defer wg.Done()
		modelsStatusCode, modelsBody, modelsErr = service.FetchAntigravityModelsQuota(ctx, client, accessToken, oauthKey.ProjectID)
	}()
	wg.Wait()

	if accountErr != nil {
		common.SysError("failed to fetch antigravity account info: " + accountErr.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取账号信息失败，请稍后重试"})
		return
	}

	var accountPayload any
	if common.Unmarshal(accountBody, &accountPayload) != nil {
		accountPayload = string(accountBody)
	}

	var modelsPayload any
	if modelsErr == nil && modelsStatusCode >= 200 && modelsStatusCode < 300 {
		if common.Unmarshal(modelsBody, &modelsPayload) != nil {
			modelsPayload = string(modelsBody)
		}
	}
	_ = modelsStatusCode

	ok := accountStatusCode >= 200 && accountStatusCode < 300
	resp := gin.H{
		"success":         ok,
		"message":         "",
		"upstream_status": accountStatusCode,
		"data":            accountPayload,
		"key_project_id":  oauthKey.ProjectID,
		"models_quota":    modelsPayload,
	}
	if !ok {
		resp["message"] = fmt.Sprintf("upstream status: %d", accountStatusCode)
	}
	c.JSON(http.StatusOK, resp)
}
