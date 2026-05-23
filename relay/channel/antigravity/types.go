package antigravity

import "time"

const (
	refreshSkew = 5 * time.Minute
)

type OAuthKey struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token,omitempty"`
	Expiry       string `json:"expiry,omitempty"`
	ProjectID    string `json:"project_id,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	Scope        string `json:"scope,omitempty"`
}
