package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	kiroCredentialRefreshTickInterval = 10 * time.Minute
	kiroCredentialRefreshThreshold    = 30 * time.Minute
	kiroCredentialRefreshBatchSize    = 200
	kiroCredentialRefreshTimeout      = 15 * time.Second
)

type KiroCredentialRefreshOptions struct {
	ResetCaches bool
}

// RefreshKiroChannelCredential 刷新单个 Kiro 渠道凭据并写回数据库（参考 codex 实现）
func RefreshKiroChannelCredential(ctx context.Context, channelID int, opts KiroCredentialRefreshOptions) (*KiroOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeKiro {
		return nil, nil, fmt.Errorf("channel type is not Kiro")
	}

	oauthKey, err := parseKiroOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, nil, fmt.Errorf("kiro channel: refresh_token is required to refresh credential")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	proxy := ch.GetSetting().Proxy
	res, err := RefreshKiroOAuthTokenWithProxy(refreshCtx, oauthKey.RefreshToken, oauthKey.AuthMethod, oauthKey.ClientID, oauthKey.ClientSecret, oauthKey.Region, proxy)
	if err != nil {
		return nil, nil, err
	}

	oauthKey.AccessToken = res.AccessToken
	oauthKey.RefreshToken = res.RefreshToken
	oauthKey.ExpiresAt = res.ExpiresAt.Format(time.RFC3339)
	if res.ProfileArn != "" {
		oauthKey.ProfileArn = res.ProfileArn
	}
	if res.Region != "" {
		oauthKey.Region = res.Region
	}
	if res.AuthMethod != "" {
		oauthKey.AuthMethod = res.AuthMethod
	}
	if res.ClientID != "" {
		oauthKey.ClientID = res.ClientID
	}
	if res.ClientSecret != "" {
		oauthKey.ClientSecret = res.ClientSecret
	}
	if res.IDCRegion != "" {
		oauthKey.IDCRegion = res.IDCRegion
	}

	encoded, err := common.Marshal(oauthKey)
	if err != nil {
		return nil, nil, err
	}

	if err := model.DB.Model(&model.Channel{}).Where("id = ?", ch.Id).Update("key", string(encoded)).Error; err != nil {
		return nil, nil, err
	}

	if opts.ResetCaches {
		model.InitChannelCache()
		ResetProxyClientCache()
	}

	return oauthKey, ch, nil
}

var (
	kiroCredentialRefreshOnce    sync.Once
	kiroCredentialRefreshRunning atomic.Bool
)

// KiroOAuthKey 定义在 service 层以避免循环导入
type KiroOAuthKey struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	AuthMethod   string `json:"auth_method,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Region       string `json:"region,omitempty"`
	ProfileArn   string `json:"profile_arn,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	IDCRegion    string `json:"idc_region,omitempty"`
}

func parseKiroOAuthKey(raw string) (*KiroOAuthKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("kiro channel: empty oauth key")
	}
	// 纯字符串格式（Social Auth refreshToken）
	if !strings.HasPrefix(raw, "{") {
		return &KiroOAuthKey{RefreshToken: raw}, nil
	}
	var key KiroOAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, fmt.Errorf("kiro channel: invalid oauth key json")
	}
	// 兼容 camelCase 格式（参考项目存储格式）
	if key.AccessToken == "" || key.RefreshToken == "" {
		var camel struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresAt    string `json:"expiresAt"`
			AuthMethod   string `json:"authMethod"`
			Region       string `json:"region"`
			ProfileArn   string `json:"profileArn"`
			ClientID     string `json:"clientId"`
			ClientSecret string `json:"clientSecret"`
			IDCRegion    string `json:"idcRegion"`
		}
		if err := common.Unmarshal([]byte(raw), &camel); err == nil {
			if key.AccessToken == "" {
				key.AccessToken = camel.AccessToken
			}
			if key.RefreshToken == "" {
				key.RefreshToken = camel.RefreshToken
			}
			if key.ExpiresAt == "" {
				key.ExpiresAt = camel.ExpiresAt
			}
			if key.AuthMethod == "" {
				key.AuthMethod = camel.AuthMethod
			}
			if key.Region == "" {
				key.Region = camel.Region
			}
			if key.ProfileArn == "" {
				key.ProfileArn = camel.ProfileArn
			}
			if key.ClientID == "" {
				key.ClientID = camel.ClientID
			}
			if key.ClientSecret == "" {
				key.ClientSecret = camel.ClientSecret
			}
			if key.IDCRegion == "" {
				key.IDCRegion = camel.IDCRegion
			}
		}
	}
	return &key, nil
}

func StartKiroCredentialAutoRefreshTask() {
	kiroCredentialRefreshOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("kiro credential auto-refresh task started: tick=%s threshold=%s", kiroCredentialRefreshTickInterval, kiroCredentialRefreshThreshold))

			ticker := time.NewTicker(kiroCredentialRefreshTickInterval)
			defer ticker.Stop()

			runKiroCredentialAutoRefreshOnce()
			for range ticker.C {
				runKiroCredentialAutoRefreshOnce()
			}
		})
	})
}

func runKiroCredentialAutoRefreshOnce() {
	if !kiroCredentialRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer kiroCredentialRefreshRunning.Store(false)

	ctx := context.Background()
	now := time.Now()

	var refreshed int
	var scanned int

	offset := 0
	for {
		var channels []*model.Channel
		err := model.DB.
			Select("id", "name", "key", "status", "channel_info").
			Where("type = ? AND (status = ? OR status = ?)",
				constant.ChannelTypeKiro,
				common.ChannelStatusEnabled,
				common.ChannelStatusAutoDisabled,
			).
			Order("id asc").
			Limit(kiroCredentialRefreshBatchSize).
			Offset(offset).
			Find(&channels).Error
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("kiro credential auto-refresh: query channels failed: %v", err))
			return
		}
		if len(channels) == 0 {
			break
		}
		offset += kiroCredentialRefreshBatchSize

		for _, ch := range channels {
			if ch == nil {
				continue
			}
			scanned++
			if ch.ChannelInfo.IsMultiKey {
				continue
			}

			rawKey := strings.TrimSpace(ch.Key)
			if rawKey == "" {
				continue
			}

			oauthKey, err := parseKiroOAuthKey(rawKey)
			if err != nil {
				continue
			}

			refreshToken := strings.TrimSpace(oauthKey.RefreshToken)
			if refreshToken == "" {
				continue
			}

			expiredAtRaw := strings.TrimSpace(oauthKey.ExpiresAt)
			expiredAt, err := time.Parse(time.RFC3339, expiredAtRaw)
			if err == nil && !expiredAt.IsZero() && expiredAt.Sub(now) > kiroCredentialRefreshThreshold {
				continue
			}

			refreshCtx, cancel := context.WithTimeout(ctx, kiroCredentialRefreshTimeout)
			newKey, refreshErr := refreshKiroChannelCredential(refreshCtx, ch.Id)
			cancel()
			if refreshErr != nil {
				logger.LogWarn(ctx, fmt.Sprintf("kiro credential auto-refresh: channel_id=%d name=%s refresh failed: %v", ch.Id, ch.Name, refreshErr))
				continue
			}

			refreshed++
			logger.LogInfo(ctx, fmt.Sprintf("kiro credential auto-refresh: channel_id=%d name=%s refreshed, expires_at=%s", ch.Id, ch.Name, newKey.ExpiresAt))
		}
	}

	if refreshed > 0 {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.LogWarn(ctx, fmt.Sprintf("kiro credential auto-refresh: InitChannelCache panic: %v", r))
				}
			}()
			model.InitChannelCache()
		}()
		ResetProxyClientCache()
	}

	if common.DebugEnabled {
		logger.LogDebug(ctx, "kiro credential auto-refresh: scanned=%d refreshed=%d", scanned, refreshed)
	}
}

func refreshKiroChannelCredential(ctx context.Context, channelID int) (*KiroOAuthKey, error) {
	key, _, err := RefreshKiroChannelCredential(ctx, channelID, KiroCredentialRefreshOptions{ResetCaches: false})
	return key, err
}
