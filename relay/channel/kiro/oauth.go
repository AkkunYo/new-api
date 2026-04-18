package kiro

import (
	"encoding/json"
	"errors"
	"strings"
)

// OAuthKey Kiro OAuth 凭证结构
type OAuthKey struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Region       string `json:"region,omitempty"`
	IDCRegion    string `json:"idc_region,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	AuthMethod   string `json:"auth_method,omitempty"`
	Provider     string `json:"provider,omitempty"`
	ProfileArn   string `json:"profile_arn,omitempty"`
}

// ParseOAuthKey 解析 OAuth 凭证，兼容 snake_case 和 camelCase 两种格式
func ParseOAuthKey(keyStr string) (*OAuthKey, error) {
	keyStr = strings.TrimSpace(keyStr)
	if keyStr == "" {
		return nil, errors.New("empty key")
	}

	// 纯字符串格式（Social Auth）
	if !strings.HasPrefix(keyStr, "{") {
		return &OAuthKey{
			RefreshToken: keyStr,
		}, nil
	}

	// JSON 格式（snake_case）
	var key OAuthKey
	if err := json.Unmarshal([]byte(keyStr), &key); err != nil {
		return nil, err
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
		if err := json.Unmarshal([]byte(keyStr), &camel); err == nil {
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

// ToJSON 转换为 JSON 字符串
func (k *OAuthKey) ToJSON() string {
	data, _ := json.Marshal(k)
	return string(data)
}
