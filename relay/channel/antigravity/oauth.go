package antigravity

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// ParseOAuthKey 解析 OAuth 凭证，兼容 snake_case 和 camelCase 两种格式
func ParseOAuthKey(keyStr string) (*OAuthKey, error) {
	keyStr = strings.TrimSpace(keyStr)
	if keyStr == "" {
		return nil, errors.New("empty key")
	}
	if !strings.HasPrefix(keyStr, "{") {
		return &OAuthKey{RefreshToken: keyStr}, nil
	}
	var key OAuthKey
	if err := common.UnmarshalJsonStr(keyStr, &key); err != nil {
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
		if err := common.UnmarshalJsonStr(keyStr, &camel); err == nil {
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

func (k *OAuthKey) ToJSON() string {
	data, _ := common.Marshal(k)
	return string(data)
}

func (k *OAuthKey) IsExpiringSoon() bool {
	if k == nil {
		return true
	}
	if strings.TrimSpace(k.Expiry) == "" {
		return true
	}
	expiry, err := time.Parse(time.RFC3339, k.Expiry)
	if err != nil {
		return true
	}
	return time.Until(expiry) <= refreshSkew
}
