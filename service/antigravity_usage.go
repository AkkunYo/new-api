package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// FetchAntigravityModelsQuota calls fetchAvailableModels and returns the raw response body.
func FetchAntigravityModelsQuota(ctx context.Context, client *http.Client, accessToken, projectID string) (int, []byte, error) {
	if client == nil {
		return 0, nil, fmt.Errorf("nil http client")
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return 0, nil, fmt.Errorf("empty access token")
	}

	endpoint := fmt.Sprintf("%s/%s:fetchAvailableModels",
		strings.TrimRight(antigravityAPIEndpoint, "/"),
		antigravityAPIVersion,
	)

	body, err := common.Marshal(map[string]any{
		"project": strings.TrimSpace(projectID),
	})
	if err != nil {
		return 0, nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "antigravity/"+antigravityVersion+" darwin/arm64")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, respBody, nil
}

// FetchAntigravityAccountInfo calls loadCodeAssist and returns the raw response body.
func FetchAntigravityAccountInfo(ctx context.Context, client *http.Client, accessToken string) (int, []byte, error) {
	if client == nil {
		return 0, nil, fmt.Errorf("nil http client")
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return 0, nil, fmt.Errorf("empty access token")
	}

	endpoint := fmt.Sprintf("%s/%s:loadCodeAssist",
		strings.TrimRight(antigravityAPIEndpoint, "/"),
		antigravityAPIVersion,
	)

	body, err := common.Marshal(map[string]any{
		"metadata": antigravityLoadCodeAssistMetadata(),
	})
	if err != nil {
		return 0, nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "antigravity/"+antigravityVersion+" darwin/arm64")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, respBody, nil
}
