package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	kiroSocialAuthEndpoint  = "https://prod.us-east-1.auth.desktop.kiro.dev"
	kiroOIDCEndpoint        = "https://oidc.us-east-1.amazonaws.com"
	kiroBuilderIDStartURL   = "https://view.awsapps.com/start"
	kiroDefaultRegion       = "us-east-1"
	kiroDefaultHTTPTimeout  = 20 * time.Second
	kiroTokenRefreshTimeout = 15 * time.Second
)

type KiroOAuthTokenResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	AuthMethod   string
	Provider     string
	Region       string
	ProfileArn   string
	ClientID     string
	ClientSecret string
	IDCRegion    string
}

type KiroSocialOAuthFlow struct {
	State        string
	Verifier     string
	Challenge    string
	AuthorizeURL string
	Provider     string
}

type KiroBuilderIDFlow struct {
	ClientID        string
	ClientSecret    string
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresIn       int
	Interval        int
}

// inferAuthMethod 自动推断认证方式
// 参考 AIClient-2-API 的实现逻辑
func inferAuthMethod(clientID, clientSecret string) string {
	hasIdcCredentials := clientID != "" && clientSecret != ""
	if hasIdcCredentials {
		return "builder-id"
	}
	return "social"
}

// RefreshKiroOAuthToken 刷新 Kiro OAuth token（自动检测 auth_method）
func RefreshKiroOAuthToken(ctx context.Context, refreshToken, authMethod, clientID, clientSecret, region string) (*KiroOAuthTokenResult, error) {
	return RefreshKiroOAuthTokenWithProxy(ctx, refreshToken, authMethod, clientID, clientSecret, region, "")
}

func RefreshKiroOAuthTokenWithProxy(ctx context.Context, refreshToken, authMethod, clientID, clientSecret, region, proxyURL string) (*KiroOAuthTokenResult, error) {
	client, err := getKiroHTTPClient(proxyURL, kiroTokenRefreshTimeout)
	if err != nil {
		return nil, err
	}

	if region == "" {
		region = kiroDefaultRegion
	}

	// 自动推断 auth_method（改进版）
	if authMethod == "" {
		authMethod = inferAuthMethod(clientID, clientSecret)
		common.SysLog(fmt.Sprintf("authMethod missing, inferred: %s", authMethod))
	}

	// 验证 builder-id 必需的凭据
	if authMethod == "builder-id" {
		if clientID == "" || clientSecret == "" {
			return nil, errors.New("builder-id auth requires clientId and clientSecret")
		}
		return refreshKiroBuilderIDToken(ctx, client, refreshToken, clientID, clientSecret, region)
	}

	return refreshKiroSocialToken(ctx, client, refreshToken, region)
}

// RefreshKiroSocialToken 刷新 Social OAuth token
func refreshKiroSocialToken(ctx context.Context, client *http.Client, refreshToken, region string) (*KiroOAuthTokenResult, error) {
	rt := strings.TrimSpace(refreshToken)
	if rt == "" {
		return nil, errors.New("empty refresh_token")
	}

	tokenURL := strings.Replace(kiroSocialAuthEndpoint, "us-east-1", region, 1) + "/refreshToken"

	payload := map[string]string{
		"refreshToken": rt,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
		ProfileArn   string `json:"profileArn"`
	}

	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kiro social oauth refresh failed: status=%d", resp.StatusCode)
	}

	if strings.TrimSpace(result.AccessToken) == "" {
		return nil, errors.New("kiro social oauth refresh response missing accessToken")
	}

	// 改进：如果响应中没有返回新的 refreshToken，使用原来的
	newRefreshToken := strings.TrimSpace(result.RefreshToken)
	if newRefreshToken == "" {
		newRefreshToken = refreshToken
	}

	// 改进：如果 expiresIn 无效，使用默认值 3600 秒
	expiresIn := result.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}

	return &KiroOAuthTokenResult{
		AccessToken:  strings.TrimSpace(result.AccessToken),
		RefreshToken: newRefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		AuthMethod:   "social",
		Region:       region,
		ProfileArn:   strings.TrimSpace(result.ProfileArn),
	}, nil
}

// RefreshKiroBuilderIDToken 刷新 Builder ID token
func refreshKiroBuilderIDToken(ctx context.Context, client *http.Client, refreshToken, clientID, clientSecret, region string) (*KiroOAuthTokenResult, error) {
	rt := strings.TrimSpace(refreshToken)
	cid := strings.TrimSpace(clientID)
	cs := strings.TrimSpace(clientSecret)

	if rt == "" {
		return nil, errors.New("empty refresh_token")
	}
	if cid == "" {
		return nil, errors.New("empty client_id")
	}
	if cs == "" {
		return nil, errors.New("empty client_secret")
	}

	tokenURL := strings.Replace(kiroOIDCEndpoint, "us-east-1", region, 1) + "/token"

	payload := map[string]string{
		"grantType":    "refresh_token",
		"refreshToken": rt,
		"clientId":     cid,
		"clientSecret": cs,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
	}

	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kiro builder-id oauth refresh failed: status=%d", resp.StatusCode)
	}

	if strings.TrimSpace(result.AccessToken) == "" {
		return nil, errors.New("kiro builder-id oauth refresh response missing accessToken")
	}

	// 改进：如果响应中没有返回新的 refreshToken，使用原来的
	newRefreshToken := strings.TrimSpace(result.RefreshToken)
	if newRefreshToken == "" {
		newRefreshToken = refreshToken
	}

	// 改进：如果 expiresIn 无效，使用默认值 3600 秒
	expiresIn := result.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}

	return &KiroOAuthTokenResult{
		AccessToken:  strings.TrimSpace(result.AccessToken),
		RefreshToken: newRefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		AuthMethod:   "builder-id",
		Region:       region,
		ClientID:     cid,
		ClientSecret: cs,
		IDCRegion:    region,
	}, nil
}

// CrteKiroSocialOAuthFlow 创建 Social OAuth 流程
func CreateKiroSocialOAuthFlow(provider, region string) (*KiroSocialOAuthFlow, error) {
	if provider != "Google" && provider != "Github" {
		return nil, errors.New("invalid provider, must be Google or Github")
	}

	state, err := createStateHex(16)
	if err != nil {
		return nil, err
	}
	verifier, challenge, err := generatePKCEPair()
	if err != nil {
		return nil, err
	}

	if region == "" {
		region = kiroDefaultRegion
	}

	authURL := strings.Replace(kiroSocialAuthEndpoint, "us-east-1", region, 1) + "/login"
	u, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("idp", provider)
	q.Set("redirect_uri", "http://127.0.0.1:19876/oauth/callback")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("prompt", "select_account")
	u.RawQuery = q.Encode()

	return &KiroSocialOAuthFlow{
		State:        state,
		Verifier:     verifier,
		Challenge:    challenge,
		AuthorizeURL: u.String(),
		Provider:     provider,
	}, nil
}

// ExchangeKiroSocialAuthCode 交换 Social OAuth 授
func ExchangeKiroSocialAuthCode(ctx context.Context, code, verifier, region string) (*KiroOAuthTokenResult, error) {
	return ExchangeKiroSocialAuthCodeWithProxy(ctx, code, verifier, region, "")
}

func ExchangeKiroSocialAuthCodeWithProxy(ctx context.Context, code, verifier, region, proxyURL string) (*KiroOAuthTokenResult, error) {
	client, err := getKiroHTTPClient(proxyURL, kiroDefaultHTTPTimeout)
	if err != nil {
		return nil, err
	}

	c := strings.TrimSpace(code)
	v := strings.TrimSpace(verifier)
	if c == "" {
		return nil, errors.New("empty authorization code")
	}
	if v == "" {
		return nil, errors.New("empty code_verifier")
	}

	if region == "" {
		region = kiroDefaultRegion
	}

	tokenURL := strings.Replace(kiroSocialAuthEndpoint, "us-east-1", region, 1) + "/oauth/token"

	payload := map[string]string{
		"code":          c,
		"code_verifier": v,
		"redirect_uri":  "http://localhost:19876/oauth/callback",
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
		ProfileArn   string `json:"profileArn"`
	}

	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kiro social oauth code exchange failed: status=%d", resp.StatusCode)
	}

	if strings.TrimSpace(result.AccessToken) == "" || strings.TrimSpace(result.RefreshToken) == "" || result.ExpiresIn <= 0 {
		return nil, errors.New("kiro social oauth token response missing fields")
	}

	return &KiroOAuthTokenResult{
		AccessToken:  strings.TrimSpace(result.AccessToken),
		RefreshToken: strings.TrimSpace(result.RefreshToken),
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
		AuthMethod:   "social",
		Region:       region,
		ProfileArn:   strings.TrimSpace(result.ProfileArn),
	}, nil
}

// CreateKiroBuilderIDFlow 创建 Builder ID 设备码流程
func CreateKiroBuilderIDFlow(ctx context.Context, region string) (*KiroBuilderIDFlow, error) {
	return CreateKiroBuilderIDFlowWithProxy(ctx, region, "")
}

func CreateKiroBuilderIDFlowWithProxy(ctx context.Context, region, proxyURL string) (*KiroBuilderIDFlow, error) {
	client, err := getKiroHTTPClient(proxyURL, kiroDefaultHTTPTimeout)
	if err != nil {
		return nil, err
	}

	if region == "" {
		region = kiroDefaultRegion
	}

	// 1. 注册 OIDC 客户端
	clientID, clientSecret, err := registerKiroOIDCClient(ctx, client, region)
	if err != nil {
		return nil, err
	}

	// 2. 请求设备授权
	deviceAuthURL := strings.Replace(kiroOIDCEndpoint, "us-east-1", region, 1) + "/device_authorization"

	payload := map[string]string{
		"clientId":     clientID,
		"clientSecret": clientSecret,
		"startUrl":     kiroBuilderIDStartURL,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceAuthURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		DeviceCode      string `json:"deviceCode"`
		UserCode        string `json:"userCode"`
		VerificationURI string `json:"verificationUri"`
		ExpiresIn       int    `json:"expiresIn"`
		Interval        int    `json:"interval"`
	}

	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kiro builder-id device authorization failed: status=%d", resp.StatusCode)
	}

	if result.Interval == 0 {
		result.Interval = 5
	}

	return &KiroBuilderIDFlow{
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		DeviceCode:      result.DeviceCode,
		UserCode:        result.UserCode,
		VerificationURI: result.VerificationURI,
		ExpiresIn:       result.ExpiresIn,
		Interval:        result.Interval,
	}, nil
}

// PollKiroBuilderIDToken 轮询 Builder ID token
func PollKiroBuilderIDToken(ctx context.Context, flow *KiroBuilderIDFlow, region string) (*KiroOAuthTokenResult, error) {
	return PollKiroBuilderIDTokenWithProxy(ctx, flow, region, "")
}

func PollKiroBuilderIDTokenWithProxy(ctx context.Context, flow *KiroBuilderIDFlow, region, proxyURL string) (*KiroOAuthTokenResult, error) {
	client, err := getKiroHTTPClient(proxyURL, kiroDefaultHTTPTimeout)
	if err != nil {
		return nil, err
	}

	if region == "" {
		region = kiroDefaultRegion
	}

	tokenURL := strings.Replace(kiroOIDCEndpoint, "us-east-1", region, 1) + "/token"

	payload := map[string]string{
		"grantType":    "urn:ietf:params:oauth:grant-type:device_code",
		"deviceCode":   flow.DeviceCode,
		"clientId":     flow.ClientID,
		"clientSecret": flow.ClientSecret,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
		Error        string `json:"error"`
	}

	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}

	// 处理轮询状态
	if result.Error != "" {
		if result.Error == "authorization_pending" {
			return nil, errors.New("authorization_pending")
		}
		if result.Error == "slow_down" {
			return nil, errors.New("slow_down")
		}
		return nil, fmt.Errorf("kiro builder-id token poll failed: %s", result.Error)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kiro builder-id token poll failed: status=%d", resp.StatusCode)
	}

	if strings.TrimSpace(result.AccessToken) == "" || strings.TrimSpace(result.RefreshToken) == "" || result.ExpiresIn <= 0 {
		return nil, errors.New("kiro builder-id token response missing fields")
	}

	return &KiroOAuthTokenResult{
		AccessToken:  strings.TrimSpace(result.AccessToken),
		RefreshToken: strings.TrimSpace(result.RefreshToken),
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
		AuthMethod:   "builder-id",
		Region:       region,
		ClientID:     flow.ClientID,
		ClientSecret: flow.ClientSecret,
		IDCRegion:    region,
	}, nil
}

// registerKiroOIDCClient 注册 OIDC 客户端
func registerKiroOIDCClient(ctx context.Context, client *http.Client, region string) (string, string, error) {
	registerURL := strings.Replace(kiroOIDCEndpoint, "us-east-1", region, 1) + "/client/register"

	payload := map[string]interface{}{
		"clientName":   "kiro-client",
		"clientType":   "public",
		"scopes":       []string{"codewhisperer:completions", "codewhisperer:analysis", "codewhisperer:conversations"},
		"grantTypes":   []string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
		"issuerUrl":    strings.Replace(kiroOIDCEndpoint, "us-east-1", region, 1),
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, strings.NewReader(string(body)))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var result struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}

	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("kiro oidc client registration failed: status=%d", resp.StatusCode)
	}

	if strings.TrimSpace(result.ClientID) == "" || strings.TrimSpace(result.ClientSecret) == "" {
		return "", "", errors.New("kiro oidc client registration response missing fields")
	}

	return strings.TrimSpace(result.ClientID), strings.TrimSpace(result.ClientSecret), nil
}

func getKiroHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	baseClient, err := GetHttpClientWithProxy(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	if baseClient == nil {
		return &http.Client{Timeout: timeout}, nil
	}
	clientCopy := *baseClient
	clientCopy.Timeout = timeout
	return &clientCopy, nil
}
