package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	antigravityOAuthAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	antigravityOAuthRedirectURI  = "http://localhost:51121/oauth-callback"
	antigravityOAuthScope        = "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/cclog https://www.googleapis.com/auth/experimentsandconfigs"
	antigravityUserInfoEndpoint  = "https://www.googleapis.com/oauth2/v2/userinfo?alt=json"
)

type AntigravityOAuthFlow struct {
	AuthorizeURL string
	State        string
}

func GenerateAntigravityOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateAntigravityOAuthFlow() (*AntigravityOAuthFlow, error) {
	state, err := GenerateAntigravityOAuthState()
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("access_type", "offline")
	params.Set("client_id", antigravityOAuthClientID)
	params.Set("prompt", "consent")
	params.Set("redirect_uri", antigravityOAuthRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", antigravityOAuthScope)
	params.Set("state", state)

	return &AntigravityOAuthFlow{
		AuthorizeURL: antigravityOAuthAuthorizeURL + "?" + params.Encode(),
		State:        state,
	}, nil
}

type AntigravityOAuthCompleteResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Email        string
	ProjectID    string
}

func CompleteAntigravityOAuthWithProxy(ctx context.Context, code, expectedState, inputState, proxyURL string) (*AntigravityOAuthCompleteResult, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("missing authorization code")
	}
	if strings.TrimSpace(expectedState) == "" {
		return nil, errors.New("oauth flow not started or session expired")
	}
	if inputState != expectedState {
		return nil, errors.New("state mismatch, possible CSRF attack")
	}

	client, err := getAntigravityHTTPClient(proxyURL, 20*time.Second)
	if err != nil {
		return nil, err
	}

	// Exchange code for tokens
	tokenResult, err := exchangeAntigravityCode(ctx, client, code)
	if err != nil {
		return nil, err
	}

	// Fetch email
	email, _ := fetchAntigravityUserEmail(ctx, client, tokenResult.AccessToken)

	// Discover project_id
	projectID, err := antigravityProjectIDDiscoverWithProxy(ctx, tokenResult.AccessToken, proxyURL)
	if err != nil {
		return nil, fmt.Errorf("project discovery failed: %w", err)
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project_id discovery returned empty")
	}

	return &AntigravityOAuthCompleteResult{
		AccessToken:  tokenResult.AccessToken,
		RefreshToken: tokenResult.RefreshToken,
		ExpiresAt:    tokenResult.ExpiresAt,
		Email:        email,
		ProjectID:    projectID,
	}, nil
}

type antigravityTokenExchangeResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func exchangeAntigravityCode(ctx context.Context, client *http.Client, code string) (*antigravityTokenExchangeResult, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", antigravityOAuthClientID)
	form.Set("client_secret", antigravityOAuthClientSecret)
	form.Set("redirect_uri", antigravityOAuthRedirectURI)
	form.Set("grant_type", "authorization_code")

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
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err = common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return nil, errors.New("token exchange returned empty access_token")
	}

	result := &antigravityTokenExchangeResult{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
	}
	if payload.ExpiresIn > 0 {
		result.ExpiresAt = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return result, nil
}

func fetchAntigravityUserEmail(ctx context.Context, client *http.Client, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, antigravityUserInfoEndpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo failed: %d", resp.StatusCode)
	}

	var info struct {
		Email string `json:"email"`
	}
	if err = common.Unmarshal(body, &info); err != nil {
		return "", err
	}
	return strings.TrimSpace(info.Email), nil
}
