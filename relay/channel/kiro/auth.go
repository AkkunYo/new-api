package kiro

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// RefreshKiroToken 刷新 Kiro Social Auth token
func RefreshKiroToken(refreshToken, region string) (accessToken, profileArn string, err error) {
	url := "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken"
	payload := map[string]string{
		"refreshToken": refreshToken,
	}

	common.SysLog(fmt.Sprintf("[Kiro] Refreshing token via %s", url))
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		common.SysLog(fmt.Sprintf("[Kiro] Token refresh request failed: %v", err))
		return "", "", err
	}
	defer resp.Body.Close()

	common.SysLog(fmt.Sprintf("[Kiro] Token refresh response: HTTP %d", resp.StatusCode))
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
		ProfileArn   string `json:"profileArn"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	common.SysLog(fmt.Sprintf("[Kiro] Token refreshed successfully, profileArn=%s", result.ProfileArn))
	return result.AccessToken, result.ProfileArn, nil
}

// RefreshKiroBuilderIDToken 刷新 Kiro Builder ID token
func RefreshKiroBuilderIDToken(refreshToken, clientId, clientSecret, region string) (accessToken, profileArn string, err error) {
	if region == "" {
		region = "us-east-1"
	}

	url := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", region)
	payload := map[string]string{
		"clientId":     clientId,
		"clientSecret": clientSecret,
		"refreshToken": refreshToken,
		"grantType":    "refresh_token",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
		ProfileArn   string `json:"profileArn"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	return result.AccessToken, result.ProfileArn, nil
}

// GenerateMachineId 生成机器 ID
func GenerateMachineId(profileArn, clientId string) string {
	seed := profileArn
	if seed == "" {
		seed = clientId
	}
	if seed == "" {
		seed = "default"
	}
	hash := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(hash[:8])
}

// ParseCredentialJSON 解析凭证 JSON
func ParseCredentialJSON(jsonStr string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := common.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, errors.New("invalid JSON format")
	}
	return result, nil
}
