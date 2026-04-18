package kiro

import "github.com/QuantumNous/new-api/common"

// KiroEndpoint Kiro API 端点配置
type KiroEndpoint struct {
	URL       string
	Origin    string
	AmzTarget string
	Name      string
}

var kiroEndpoints = []KiroEndpoint{
	{
		URL:       "https://q.us-east-1.amazonaws.com/generateAssistantResponse",
		Origin:    "AI_EDITOR",
		AmzTarget: "",
		Name:      "Kiro IDE",
	},
	{
		URL:       "https://codewhisperer.us-east-1.amazonaws.com/generateAssistantResponse",
		Origin:    "AI_EDITOR",
		AmzTarget: "AmazonCodeWhispererStreamingService.GenerateAssistantResponse",
		Name:      "CodeWhisperer",
	},
	{
		URL:       "https://q.us-east-1.amazonaws.com/generateAssistantResponse",
		Origin:    "AI_EDITOR",
		AmzTarget: "AmazonQDeveloperStreamingService.SendMessage",
		Name:      "AmazonQ",
	},
}

// GetSortedEndpoints 获取排序后的端点列表
func GetSortedEndpoints(preferred string, fallback bool) []KiroEndpoint {
	if preferred == "auto" || fallback {
		return kiroEndpoints
	}
	// 简化实现：总是返回所有端点
	return kiroEndpoints
}

// ShouldRetryOnError 判断是否应该重试
func ShouldRetryOnError(statusCode int) bool {
	// 429 (quota) 和 5xx (server error) 可重试
	// 401/403 (auth) 不可重试
	return statusCode == 429 || (statusCode >= 500 && statusCode < 600)
}

// LogEndpointSwitch 记录端点切换
func LogEndpointSwitch(from, to, reason string) {
	common.SysLog("[Kiro] Switching from " + from + " to " + to + " (" + reason + ")")
}
