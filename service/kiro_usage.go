package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/google/uuid"
)

// generateMachineId 根据配置生成唯一的机器码
func generateMachineId(profileArn, clientId string) string {
	uniqueKey := profileArn
	if uniqueKey == "" {
		uniqueKey = clientId
	}
	if uniqueKey == "" {
		uniqueKey = "KIRO_DEFAULT_MACHINE"
	}
	hash := sha256.Sum256([]byte(uniqueKey))
	return fmt.Sprintf("%x", hash)
}

// getSystemRuntimeInfo 获取系统运行时信息
func getSystemRuntimeInfo() (osName, nodeVersion string) {
	osName = runtime.GOOS
	nodeVersion = runtime.Version()
	// 移除 "go" 前缀
	nodeVersion = strings.TrimPrefix(nodeVersion, "go")
	return
}

// FetchKiroUsageLimits 获取 Kiro 渠道的用量限制信息
func FetchKiroUsageLimits(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	accessToken string,
	profileArn string,
	isBuilderID bool,
	clientId string,
) (statusCode int, body []byte, err error) {
	if client == nil {
		return 0, nil, fmt.Errorf("nil http client")
	}

	bu := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if bu == "" {
		return 0, nil, fmt.Errorf("empty baseURL")
	}

	at := strings.TrimSpace(accessToken)
	if at == "" {
		return 0, nil, fmt.Errorf("empty accessToken")
	}

	// 构建 URL：baseURL + /getUsageLimits
	// 如果 baseURL 已经包含 /generateAssistantResponse，则替换为 /getUsageLimits
	// 否则直接追加 /getUsageLimits
	var usageURL string
	if strings.Contains(bu, "/generateAssistantResponse") {
		usageURL = strings.Replace(bu, "/generateAssistantResponse", "/getUsageLimits", 1)
	} else {
		usageURL = bu + "/getUsageLimits"
	}

	// 构建查询参数
	params := url.Values{}
	params.Add("isEmailRequired", "true")
	params.Add("origin", "AI_EDITOR")
	params.Add("resourceType", "AGENTIC_REQUEST")

	// Social Auth 需要 profileArn
	if !isBuilderID && profileArn != "" {
		params.Add("profileArn", profileArn)
	}

	fullURL := usageURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return 0, nil, err
	}

	// 生成 machineId
	machineId := generateMachineId(profileArn, clientId)
	kiroVersion := "0.11.63"
	osName, goVersion := getSystemRuntimeInfo()

	// 设置 headers（参考 AIClient-2-API 的实现）
	req.Header.Set("Authorization", "Bearer "+at)
	req.Header.Set("x-amz-user-agent", fmt.Sprintf("aws-sdk-js/1.0.34 KiroIDE-%s-%s", kiroVersion, machineId))
	req.Header.Set("user-agent", fmt.Sprintf("aws-sdk-js/1.0.34 ua/2.1 os/%s lang/go md/go#%s api/codewhispererstreaming#1.0.34 m/E KiroIDE-%s-%s", osName, goVersion, kiroVersion, machineId))
	req.Header.Set("amz-sdk-invocation-id", uuid.New().String())
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("Connection", "close")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, body, nil
}
