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
	"github.com/QuantumNous/new-api/relay/channel/kiro"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type kiroOAuthStartRequest struct {
	Region string `json:"region"` // "us-east-1"
}

type kiroBuilderIDPollRequest struct {
	TaskID string `json:"task_id"`
}

// handleKiroPollingResult 处理后台轮询结果
func handleKiroPollingResult(taskID string, channelID int, task *service.KiroPollingTask) {
	select {
	case result := <-task.ResultChan:
		// 授权成功，保存凭证
		if channelID > 0 {
			key := kiro.OAuthKey{
				AccessToken:  result.AccessToken,
				RefreshToken: result.RefreshToken,
				ExpiresAt:    result.ExpiresAt.Format(time.RFC3339),
				AuthMethod:   "builder-id",
				Provider:     "BuilderId",
				Region:       result.Region,
				ClientID:     result.ClientID,
				ClientSecret: result.ClientSecret,
				IDCRegion:    result.IDCRegion,
			}

			keyJSON, err := common.Marshal(key)
			if err != nil {
				common.SysError(fmt.Sprintf("Kiro OAuth: 序列化凭证失败 [%s]: %v", taskID, err))
				return
			}

			err = model.DB.Model(&model.Channel{}).
				Where("id = ?", channelID).
				Update("key", string(keyJSON)).Error
			if err != nil {
				common.SysError(fmt.Sprintf("Kiro OAuth: 保存凭证失败 [%s]: %v", taskID, err))
				return
			}

			common.SysLog(fmt.Sprintf("Kiro OAuth: 凭证已保存到渠道 %d [%s]", channelID, taskID))
			model.InitChannelCache()
		}

	case err := <-task.ErrorChan:
		common.SysError(fmt.Sprintf("Kiro OAuth: 授权失败 [%s]: %v", taskID, err))
	}
}

// StartKiroBuilderIDOAuth 启动 Builder ID OAuth 流程
func StartKiroBuilderIDOAuth(c *gin.Context) {
	startKiroBuilderIDOAuthWithChannelID(c, 0)
}

func StartKiroBuilderIDOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	startKiroBuilderIDOAuthWithChannelID(c, channelID)
}

func startKiroBuilderIDOAuthWithChannelID(c *gin.Context, channelID int) {
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
		if ch.Type != constant.ChannelTypeKiro {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Kiro"})
			return
		}
		channelProxy = ch.GetSetting().Proxy
	}

	req := kiroOAuthStartRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	region := strings.TrimSpace(req.Region)
	if region == "" {
		region = "us-east-1"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	flow, err := service.CreateKiroBuilderIDFlowWithProxy(ctx, region, channelProxy)
	if err != nil {
		common.SysError("failed to create kiro builder-id flow: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "创建 Builder ID 授权流程失败，请重试"})
		return
	}

	// 生成任务 ID
	taskID := fmt.Sprintf("kiro-%d-%d", channelID, time.Now().Unix())

	// 创建轮询任务
	task := &service.KiroPollingTask{
		TaskID:       taskID,
		ClientID:     flow.ClientID,
		ClientSecret: flow.ClientSecret,
		DeviceCode:   flow.DeviceCode,
		Region:       region,
		ProxyURL:     channelProxy,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Duration(flow.ExpiresIn) * time.Second),
		ResultChan:   make(chan *service.KiroOAuthTokenResult, 1),
		ErrorChan:    make(chan error, 1),
	}

	// 启动后台轮询
	manager := service.GetKiroOAuthManager()
	manager.StartPollingTask(task)

	// 启动 goroutine 处理轮询结果
	go handleKiroPollingResult(taskID, channelID, task)

	// 构造完整的验证 URL（包含 user_code）
	verificationUriComplete := flow.VerificationURI
	if flow.UserCode != "" {
		verificationUriComplete = flow.VerificationURI + "?user_code=" + flow.UserCode
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"task_id":                   taskID,
			"user_code":                 flow.UserCode,
			"verification_uri":          flow.VerificationURI,
			"verification_uri_complete": verificationUriComplete,
			"expires_in":                flow.ExpiresIn,
			"interval":                  flow.Interval,
		},
	})
}

// PollKiroBuilderIDToken 轮询 Builder ID token
func PollKiroBuilderIDToken(c *gin.Context) {
	pollKiroBuilderIDTokenWithChannelID(c, 0)
}

func PollKiroBuilderIDTokenForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	pollKiroBuilderIDTokenWithChannelID(c, channelID)
}

func pollKiroBuilderIDTokenWithChannelID(c *gin.Context, channelID int) {
	req := kiroBuilderIDPollRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	taskID := strings.TrimSpace(req.TaskID)
	if taskID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "missing task_id"})
		return
	}

	manager := service.GetKiroOAuthManager()
	task, exists := manager.GetTask(taskID)
	if !exists {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "task not found or expired"})
		return
	}

	// 先检查缓存的结果
	task.Mu.RLock()
	cachedResult := task.Result
	cachedError := task.Error
	task.Mu.RUnlock()

	if cachedResult != nil {
		// 已有缓存的成功结果
		key := kiro.OAuthKey{
			AccessToken:  cachedResult.AccessToken,
			RefreshToken: cachedResult.RefreshToken,
			ExpiresAt:    cachedResult.ExpiresAt.Format(time.RFC3339),
			AuthMethod:   "builder-id",
			Provider:     "BuilderId",
			Region:       cachedResult.Region,
			ClientID:     cachedResult.ClientID,
			ClientSecret: cachedResult.ClientSecret,
			IDCRegion:    cachedResult.IDCRegion,
		}

		keyJSON, err := common.Marshal(key)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "OAuth 授权成功",
			"data": gin.H{
				"key":        string(keyJSON),
				"expires_at": cachedResult.ExpiresAt.Format(time.RFC3339),
			},
		})
		return
	}

	if cachedError != nil {
		// 已有缓存的错误结果
		c.JSON(http.StatusOK, gin.H{"success": false, "message": cachedError.Error()})
		return
	}

	// 没有缓存结果，非阻塞检查 channel
	select {
	case result := <-task.ResultChan:
		// 授权成功
		key := kiro.OAuthKey{
			AccessToken:  result.AccessToken,
			RefreshToken: result.RefreshToken,
			ExpiresAt:    result.ExpiresAt.Format(time.RFC3339),
			AuthMethod:   "builder-id",
			Provider:     "BuilderId",
			Region:       result.Region,
			ClientID:     result.ClientID,
			ClientSecret: result.ClientSecret,
			IDCRegion:    result.IDCRegion,
		}

		keyJSON, err := common.Marshal(key)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "OAuth 授权成功",
			"data": gin.H{
				"key":        string(keyJSON),
				"expires_at": result.ExpiresAt.Format(time.RFC3339),
			},
		})

	case err := <-task.ErrorChan:
		// 授权失败
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})

	default:
		// 仍在等待
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "pending",
			"data": gin.H{
				"status": "pending",
			},
		})
	}
}

// RefreshKiroChannelCredential 手动刷新渠道凭证
func RefreshKiroChannelCredential(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelID, true)
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

	oauthKey, err := kiro.ParseOAuthKey(ch.Key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid oauth key"})
		return
	}

	refreshToken := strings.TrimSpace(oauthKey.RefreshToken)
	if refreshToken == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "refresh_token is empty"})
		return
	}

	authMethod := strings.TrimSpace(oauthKey.AuthMethod)
	clientID := strings.TrimSpace(oauthKey.ClientID)
	clientSecret := strings.TrimSpace(oauthKey.ClientSecret)
	region := strings.TrimSpace(oauthKey.Region)
	if region == "" {
		region = "us-east-1"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	channelProxy := ch.GetSetting().Proxy
	tokenRes, err := service.RefreshKiroOAuthTokenWithProxy(ctx, refreshToken, authMethod, clientID, clientSecret, region, channelProxy)
	if err != nil {
		common.SysError("failed to refresh kiro token: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "刷新 token 失败，请检查凭证是否有效"})
		return
	}

	newKey := kiro.OAuthKey{
		AccessToken:  tokenRes.AccessToken,
		RefreshToken: tokenRes.RefreshToken,
		ExpiresAt:    tokenRes.ExpiresAt.Format(time.RFC3339),
		AuthMethod:   tokenRes.AuthMethod,
		Provider:     oauthKey.Provider,
		Region:       tokenRes.Region,
		ProfileArn:   tokenRes.ProfileArn,
		ClientID:     tokenRes.ClientID,
		ClientSecret: tokenRes.ClientSecret,
		IDCRegion:    tokenRes.IDCRegion,
	}

	newKeyJSON, err := common.Marshal(newKey)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	err = model.DB.Model(&model.Channel{}).
		Where("id = ?", channelID).
		Update("key", string(newKeyJSON)).Error
	if err != nil {
		common.ApiError(c, err)
		return
	}

	model.InitChannelCache()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "凭证刷新成功",
		"data": gin.H{
			"expires_at": tokenRes.ExpiresAt.Format(time.RFC3339),
		},
	})
}
