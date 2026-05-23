package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	antigravityOAuthClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityOAuthClientSecret = "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"
	antigravityAPIVersion        = "v1internal"
	antigravityOnboardMaxAttempt = 5
	antigravityVersion           = "1.24.0"
)

var (
	antigravityOAuthTokenEndpoint         = "https://oauth2.googleapis.com/token"
	antigravityAPIEndpoint                = "https://cloudcode-pa.googleapis.com"
	antigravityDailyAPIEndpoint           = "https://daily-cloudcode-pa.googleapis.com"
	antigravityOAuthRefreshWithProxy      = RefreshAntigravityOAuthTokenWithProxy
	antigravityProjectIDDiscoverWithProxy = DiscoverAntigravityProjectIDWithProxy
)

type AntigravityCredentialRefreshOptions struct {
	ResetCaches bool
}

type AntigravityOAuthKey struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token,omitempty"`
	Expiry       string `json:"expiry,omitempty"`
	ProjectID    string `json:"project_id,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type AntigravityOAuthTokenResult struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	TokenType    string
	Scope        string
}

func parseAntigravityOAuthKey(raw string) (*AntigravityOAuthKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty key")
	}
	if !strings.HasPrefix(raw, "{") {
		return &AntigravityOAuthKey{RefreshToken: raw}, nil
	}
	var key AntigravityOAuthKey
	if err := common.UnmarshalJsonStr(raw, &key); err != nil {
		return nil, err
	}

	// 兼容 camelCase 格式
	if key.AccessToken == "" || key.RefreshToken == "" {
		var camel struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			Expiry       string `json:"expiry"`
			ProjectID    string `json:"projectId"`
			TokenType    string `json:"tokenType"`
			Scope        string `json:"scope"`
		}
		if err := common.UnmarshalJsonStr(raw, &camel); err == nil {
			if key.AccessToken == "" {
				key.AccessToken = camel.AccessToken
			}
			if key.RefreshToken == "" {
				key.RefreshToken = camel.RefreshToken
			}
			if key.Expiry == "" {
				key.Expiry = camel.Expiry
			}
			if key.ProjectID == "" {
				key.ProjectID = camel.ProjectID
			}
			if key.TokenType == "" {
				key.TokenType = camel.TokenType
			}
			if key.Scope == "" {
				key.Scope = camel.Scope
			}
		}
	}

	return &key, nil
}

func RefreshAntigravityChannelCredential(ctx context.Context, channelID int, opts AntigravityCredentialRefreshOptions) (*AntigravityOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeAntigravity {
		return nil, nil, fmt.Errorf("channel type is not Antigravity")
	}

	oauthKey, err := parseAntigravityOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, nil, errors.New("refresh_token is required")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	proxy := ch.GetSetting().Proxy
	res, err := antigravityOAuthRefreshWithProxy(refreshCtx, oauthKey.RefreshToken, proxy)
	if err != nil {
		return nil, nil, err
	}

	oauthKey.AccessToken = strings.TrimSpace(res.AccessToken)
	if strings.TrimSpace(res.RefreshToken) != "" {
		oauthKey.RefreshToken = strings.TrimSpace(res.RefreshToken)
	}
	if !res.Expiry.IsZero() {
		oauthKey.Expiry = res.Expiry.Format(time.RFC3339)
	}
	if strings.TrimSpace(res.TokenType) != "" {
		oauthKey.TokenType = strings.TrimSpace(res.TokenType)
	}
	if strings.TrimSpace(res.Scope) != "" {
		oauthKey.Scope = strings.TrimSpace(res.Scope)
	}

	if strings.TrimSpace(oauthKey.ProjectID) == "" {
		projectID, err := antigravityProjectIDDiscoverWithProxy(refreshCtx, oauthKey.AccessToken, proxy)
		if err != nil {
			return nil, nil, err
		}
		oauthKey.ProjectID = strings.TrimSpace(projectID)
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

func RefreshAntigravityOAuthTokenWithProxy(ctx context.Context, refreshToken string, proxyURL string) (*AntigravityOAuthTokenResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, errors.New("refresh_token is required")
	}
	client, err := getAntigravityHTTPClient(proxyURL, 10*time.Second)
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", antigravityOAuthClientID)
	form.Set("client_secret", antigravityOAuthClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, antigravityOAuthTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("antigravity oauth refresh failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}
	if err = common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return nil, errors.New("antigravity oauth refresh missing access_token")
	}

	result := &AntigravityOAuthTokenResult{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		TokenType:    strings.TrimSpace(payload.TokenType),
		Scope:        strings.TrimSpace(payload.Scope),
	}
	if payload.ExpiresIn > 0 {
		result.Expiry = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return result, nil
}

func DiscoverAntigravityProjectIDWithProxy(ctx context.Context, accessToken string, proxyURL string) (string, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return "", errors.New("access_token is required")
	}
	client, err := getAntigravityHTTPClient(proxyURL, 10*time.Second)
	if err != nil {
		return "", err
	}

	loadEndpoint := fmt.Sprintf("%s/%s:loadCodeAssist", strings.TrimRight(antigravityAPIEndpoint, "/"), antigravityAPIVersion)
	loadBody := map[string]any{
		"metadata": antigravityLoadCodeAssistMetadata(),
	}
	loadResp, err := doAntigravityJSONRequest(ctx, client, loadEndpoint, accessToken, loadBody)
	if err != nil {
		return "", err
	}
	if projectID := extractAntigravityProjectID(loadResp); projectID != "" {
		return projectID, nil
	}

	tierID := extractAntigravityTierID(loadResp)
	onboardEndpoint := fmt.Sprintf("%s/%s:onboardUser", strings.TrimRight(antigravityDailyAPIEndpoint, "/"), antigravityAPIVersion)
	onboardBody := map[string]any{
		"tier_id":  tierID,
		"metadata": antigravityControlPlaneMetadata(),
	}
	for attempt := 1; attempt <= antigravityOnboardMaxAttempt; attempt++ {
		onboardResp, requestErr := doAntigravityJSONRequest(ctx, client, onboardEndpoint, accessToken, onboardBody)
		if requestErr != nil {
			return "", requestErr
		}
		if done, ok := onboardResp["done"].(bool); ok && done {
			// CPA extracts project_id from the nested "response" field first
			if responseData, ok := onboardResp["response"].(map[string]any); ok {
				if projectID := extractAntigravityProjectID(responseData); projectID != "" {
					return projectID, nil
				}
			}
			if projectID := extractAntigravityProjectID(onboardResp); projectID != "" {
				return projectID, nil
			}
			return "", errors.New("antigravity onboard user missing project id")
		}
		if attempt < antigravityOnboardMaxAttempt {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	return "", fmt.Errorf("antigravity onboard user did not complete after %d attempts", antigravityOnboardMaxAttempt)
}

func getAntigravityHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	client, err := GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return http.DefaultClient, nil
	}
	clone := *client
	if timeout > 0 {
		clone.Timeout = timeout
	}
	return &clone, nil
}

func doAntigravityJSONRequest(ctx context.Context, client *http.Client, endpoint string, accessToken string, body map[string]any) (map[string]any, error) {
	encoded, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(encoded)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("antigravity request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var payload map[string]any
	if len(respBody) == 0 {
		return map[string]any{}, nil
	}
	if err = common.Unmarshal(respBody, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func extractAntigravityProjectID(payload map[string]any) string {
	return firstNonEmptyString(
		readNestedString(payload, "cloudaicompanionProject"),
		readNestedString(payload, "cloudAiCompanionProject"),
		readNestedString(payload, "projectId"),
		readNestedString(payload, "project_id"),
		readNestedString(payload, "project"),
		readNestedString(payload, "project", "id"),
		readNestedString(payload, "data", "cloudaicompanionProject"),
		readNestedString(payload, "data", "cloudAiCompanionProject"),
		readNestedString(payload, "data", "projectId"),
		readNestedString(payload, "data", "project"),
		readNestedString(payload, "data", "project", "id"),
	)
}

func extractAntigravityTierID(payload map[string]any) string {
	if tierID := defaultAntigravityTierID(payload); tierID != "" {
		return tierID
	}
	return firstNonEmptyString(
		readNestedString(payload, "tierId"),
		readNestedString(payload, "tier_id"),
		readNestedString(payload, "defaultTierId"),
		readNestedString(payload, "data", "tierId"),
		"free-tier",
	)
}

func defaultAntigravityTierID(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if tiers, ok := payload["allowedTiers"].([]any); ok {
		for _, rawTier := range tiers {
			tier, ok := rawTier.(map[string]any)
			if !ok {
				continue
			}
			isDefault, ok := tier["isDefault"].(bool)
			if !ok || !isDefault {
				continue
			}
			if id, ok := tier["id"].(string); ok && strings.TrimSpace(id) != "" {
				return strings.TrimSpace(id)
			}
		}
	}
	return readNestedString(payload, "currentTier", "id")
}

func antigravityLoadCodeAssistMetadata() map[string]any {
	return map[string]any{"ideType": "ANTIGRAVITY"}
}

func antigravityControlPlaneMetadata() map[string]any {
	return map[string]any{
		"ide_type":    "ANTIGRAVITY",
		"ide_version": antigravityVersion,
		"ide_name":    "antigravity",
	}
}

func readNestedString(payload map[string]any, keys ...string) string {
	current := any(payload)
	for _, key := range keys {
		nextMap, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = nextMap[key]
		if !ok {
			return ""
		}
	}
	value, ok := current.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

const (
	antigravityCredentialRefreshTickInterval = 10 * time.Minute
	antigravityCredentialRefreshThreshold    = 10 * time.Minute
	antigravityCredentialRefreshBatchSize    = 200
	antigravityCredentialRefreshTimeout      = 15 * time.Second
)

var (
	antigravityCredentialRefreshOnce    sync.Once
	antigravityCredentialRefreshRunning atomic.Bool
)

func StartAntigravityCredentialAutoRefreshTask() {
	antigravityCredentialRefreshOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("antigravity credential auto-refresh task started: tick=%s threshold=%s", antigravityCredentialRefreshTickInterval, antigravityCredentialRefreshThreshold))

			ticker := time.NewTicker(antigravityCredentialRefreshTickInterval)
			defer ticker.Stop()

			runAntigravityCredentialAutoRefreshOnce()
			for range ticker.C {
				runAntigravityCredentialAutoRefreshOnce()
			}
		})
	})
}

func runAntigravityCredentialAutoRefreshOnce() {
	if !antigravityCredentialRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer antigravityCredentialRefreshRunning.Store(false)

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
				constant.ChannelTypeAntigravity,
				common.ChannelStatusEnabled,
				common.ChannelStatusAutoDisabled,
			).
			Order("id asc").
			Limit(antigravityCredentialRefreshBatchSize).
			Offset(offset).
			Find(&channels).Error
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("antigravity credential auto-refresh: query channels failed: %v", err))
			return
		}
		if len(channels) == 0 {
			break
		}
		offset += antigravityCredentialRefreshBatchSize

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

			oauthKey, err := parseAntigravityOAuthKey(rawKey)
			if err != nil {
				continue
			}

			refreshToken := strings.TrimSpace(oauthKey.RefreshToken)
			if refreshToken == "" {
				continue
			}

			expiryRaw := strings.TrimSpace(oauthKey.Expiry)
			expiry, err := time.Parse(time.RFC3339, expiryRaw)
			if err == nil && !expiry.IsZero() && expiry.Sub(now) > antigravityCredentialRefreshThreshold {
				continue
			}

			refreshCtx, cancel := context.WithTimeout(ctx, antigravityCredentialRefreshTimeout)
			newKey, _, err := RefreshAntigravityChannelCredential(refreshCtx, ch.Id, AntigravityCredentialRefreshOptions{ResetCaches: false})
			cancel()
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("antigravity credential auto-refresh: channel_id=%d name=%s refresh failed: %v", ch.Id, ch.Name, err))
				continue
			}

			refreshed++
			logger.LogInfo(ctx, fmt.Sprintf("antigravity credential auto-refresh: channel_id=%d name=%s refreshed, expires_at=%s", ch.Id, ch.Name, newKey.Expiry))
		}
	}

	if refreshed > 0 {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.LogWarn(ctx, fmt.Sprintf("antigravity credential auto-refresh: InitChannelCache panic: %v", r))
				}
			}()
			model.InitChannelCache()
		}()
		ResetProxyClientCache()
	}

	if common.DebugEnabled {
		logger.LogDebug(ctx, "antigravity credential auto-refresh: scanned=%d refreshed=%d", scanned, refreshed)
	}
}
