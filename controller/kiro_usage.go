package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetKiroChannelUsage 获取 Kiro 渠道的用量信息
func GetKiroChannelUsage(c *gin.Context) {
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
	if ch.Type != constant.ChannelTypeKiro {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Kiro"})
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	key := strings.TrimSpace(ch.Key)

	// 解析密钥
	var refreshToken string
	var region string
	var clientId string
	var clientSecret string
	var authMethod string

	if strings.HasPrefix(key, "{") {
		// JSON 格式
		var keyData map[string]interface{}
		if err := common.Unmarshal([]byte(key), &keyData); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析密钥失败，请检查渠道配置"})
			return
		}

		if rt, ok := keyData["refresh_token"].(string); ok {
			refreshToken = strings.TrimSpace(rt)
		} else if rt, ok := keyData["refreshToken"].(string); ok {
			refreshToken = strings.TrimSpace(rt)
		}

		if r, ok := keyData["region"].(string); ok {
			region = strings.TrimSpace(r)
		}

		if r, ok := keyData["idc_region"].(string); ok {
			region = strings.TrimSpace(r)
		} else if r, ok := keyData["idcRegion"].(string); ok {
			region = strings.TrimSpace(r)
		}

		if cid, ok := keyData["client_id"].(string); ok {
			clientId = strings.TrimSpace(cid)
		} else if cid, ok := keyData["clientId"].(string); ok {
			clientId = strings.TrimSpace(cid)
		}

		if cs, ok := keyData["client_secret"].(string); ok {
			clientSecret = strings.TrimSpace(cs)
		} else if cs, ok := keyData["clientSecret"].(string); ok {
			clientSecret = strings.TrimSpace(cs)
		}

		if am, ok := keyData["auth_method"].(string); ok {
			authMethod = strings.TrimSpace(am)
		} else if am, ok := keyData["authMethod"].(string); ok {
			authMethod = strings.TrimSpace(am)
		}
	} else {
		// 纯字符串格式（默认 Social Auth）
		refreshToken = key
	}

	if refreshToken == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "refresh_token is required"})
		return
	}

	// 判断认证方式
	isBuilderID := clientId != "" && clientSecret != ""
	if authMethod == "builder-id" {
		isBuilderID = true
	}

	// 创建 HTTP 客户端（需要在刷新 token 之前创建，因为刷新也需要代理）
	channelProxy := ch.GetSetting().Proxy

	// 刷新 token 获取 accessToken 和 profileArn
	refreshCtx, refreshCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer refreshCancel()

	tokenRes, err := service.RefreshKiroOAuthTokenWithProxy(refreshCtx, refreshToken, authMethod, clientId, clientSecret, region, channelProxy)
	if err != nil {
		common.SysError("failed to refresh kiro token: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "刷新 token 失败，请检查渠道配置"})
		return
	}

	accessToken := tokenRes.AccessToken
	profileArn := tokenRes.ProfileArn

	// 创建 HTTP 客户端用于查询用量
	client, err := service.NewProxyHttpClient(channelProxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	// 获取用量信息
	statusCode, body, err := service.FetchKiroUsageLimits(ctx, client, ch.GetBaseURL(), accessToken, profileArn, isBuilderID, clientId)
	if err != nil {
		common.SysError("failed to fetch kiro usage: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取用量信息失败，请稍后重试"})
		return
	}

	var payload any
	if common.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}

	ok := statusCode >= 200 && statusCode < 300
	resp := gin.H{
		"success":         ok,
		"message":         "",
		"upstream_status": statusCode,
		"data":            payload,
	}
	if !ok {
		resp["message"] = fmt.Sprintf("upstream status: %d", statusCode)
	}
	c.JSON(http.StatusOK, resp)
}
