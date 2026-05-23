package antigravity

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/google/uuid"
)

const antigravityToolSignatureSkip = "skip_thought_signature_validator"

var (
	randSource      = rand.New(rand.NewSource(time.Now().UnixNano()))
	randSourceMutex sync.Mutex
)

type AntigravityRequest struct {
	Model       string                  `json:"model"`
	Project     string                  `json:"project"`
	RequestID   string                  `json:"requestId"`
	UserAgent   string                  `json:"userAgent"`
	RequestType string                  `json:"requestType"`
	Request     AntigravityInnerRequest `json:"request"`
}

type AntigravityInnerRequest struct {
	SessionID         string                 `json:"sessionId"`
	Contents          []AntigravityContent   `json:"contents"`
	GenerationConfig  map[string]any         `json:"generationConfig,omitempty"`
	SystemInstruction *AntigravityContent    `json:"systemInstruction,omitempty"`
	Tools             []AntigravityTool      `json:"tools,omitempty"`
	ToolConfig        *AntigravityToolConfig `json:"toolConfig,omitempty"`
}

type AntigravityContent struct {
	Role  string            `json:"role"`
	Parts []AntigravityPart `json:"parts"`
}

type AntigravityPart struct {
	Text             string                       `json:"text,omitempty"`
	InlineData       *AntigravityInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *AntigravityFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *AntigravityFunctionResponse `json:"functionResponse,omitempty"`
	Thought          bool                         `json:"thought,omitempty"`
	ThoughtSignature string                       `json:"thoughtSignature,omitempty"`
}

type AntigravityInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type AntigravityFunctionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type AntigravityFunctionResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type AntigravityTool struct {
	FunctionDeclarations []AntigravityFunctionDeclaration `json:"functionDeclarations"`
}

type AntigravityFunctionDeclaration struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type AntigravityToolConfig struct {
	FunctionCallingConfig AntigravityFunctionCallingConfig `json:"functionCallingConfig"`
}

type AntigravityFunctionCallingConfig struct {
	Mode                 string   `json:"mode"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

func TranslateOpenAIToAntigravity(request *dto.GeneralOpenAIRequest, projectID string) (*AntigravityRequest, error) {
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = "gemini-2.0-flash-exp"
	}

	toolNames := make(map[string]string)
	contents := make([]AntigravityContent, 0, len(request.Messages))
	var systemInstruction *AntigravityContent

	for _, msg := range request.Messages {
		role := strings.TrimSpace(msg.Role)
		switch role {
		case "system", "developer":
			parts := antigravityMessageParts(msg)
			if len(parts) == 0 {
				continue
			}
			if systemInstruction == nil {
				systemInstruction = &AntigravityContent{Role: "user", Parts: parts}
			} else {
				systemInstruction.Parts = append(systemInstruction.Parts, parts...)
			}
		case "assistant":
			parts, err := antigravityAssistantParts(msg, toolNames)
			if err != nil {
				return nil, err
			}
			if len(parts) == 0 {
				parts = antigravityMessageParts(msg)
			}
			contents = append(contents, AntigravityContent{Role: "model", Parts: parts})
		case "tool":
			contents = append(contents, AntigravityContent{Role: "user", Parts: antigravityToolResponseParts(msg, toolNames)})
		default:
			parts := antigravityMessageParts(msg)
			if len(parts) == 0 {
				continue
			}
			if role == "" {
				role = "user"
			}
			contents = append(contents, AntigravityContent{Role: role, Parts: parts})
		}
	}

	generationConfig := antigravityGenerationConfig(request, model)
	tools := antigravityTools(request.Tools)
	toolConfig := antigravityToolConfig(request, model, tools)

	return &AntigravityRequest{
		Model:       model,
		Project:     projectID,
		RequestID:   generateRequestID(),
		UserAgent:   "antigravity",
		RequestType: "agent",
		Request: AntigravityInnerRequest{
			SessionID:         generateStableSessionID(request),
			Contents:          contents,
			GenerationConfig:  generationConfig,
			SystemInstruction: systemInstruction,
			Tools:             tools,
			ToolConfig:        toolConfig,
		},
	}, nil
}

func antigravityMessageParts(msg dto.Message) []AntigravityPart {
	parts := make([]AntigravityPart, 0)
	if reasoning := strings.TrimSpace(msg.GetReasoningContent()); reasoning != "" {
		parts = append(parts, AntigravityPart{Text: reasoning, Thought: true})
	}
	for _, content := range msg.ParseContent() {
		switch content.Type {
		case dto.ContentTypeText:
			if content.Text != "" {
				parts = append(parts, AntigravityPart{Text: content.Text})
			}
		case dto.ContentTypeImageURL:
			if inline := antigravityInlineDataFromImage(content); inline != nil {
				parts = append(parts, AntigravityPart{InlineData: inline, ThoughtSignature: antigravityToolSignatureSkip})
			}
		case dto.ContentTypeFile:
			if inline := antigravityInlineDataFromFile(content); inline != nil {
				parts = append(parts, AntigravityPart{InlineData: inline})
			}
		case dto.ContentTypeInputAudio:
			if inline := antigravityInlineDataFromAudio(content); inline != nil {
				parts = append(parts, AntigravityPart{InlineData: inline})
			}
		}
	}
	return parts
}

func antigravityAssistantParts(msg dto.Message, toolNames map[string]string) ([]AntigravityPart, error) {
	parts := antigravityMessageParts(msg)
	for _, toolCall := range msg.ParseToolCalls() {
		if toolCall.Type != "" && toolCall.Type != "function" {
			continue
		}
		name := strings.TrimSpace(toolCall.Function.Name)
		if name == "" {
			continue
		}
		if toolCall.ID != "" {
			toolNames[toolCall.ID] = name
		}
		args := map[string]any{}
		if strings.TrimSpace(toolCall.Function.Arguments) != "" {
			if err := common.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return nil, err
			}
		}
		parts = append(parts, AntigravityPart{
			ThoughtSignature: antigravityToolSignatureSkip,
			FunctionCall: &AntigravityFunctionCall{
				ID:   toolCall.ID,
				Name: name,
				Args: args,
			},
		})
	}
	return parts, nil
}

func antigravityToolResponseParts(msg dto.Message, toolNames map[string]string) []AntigravityPart {
	name := toolNames[msg.ToolCallId]
	if name == "" {
		name = strings.TrimSpace(msg.ToolCallId)
	}
	if name == "" {
		name = "tool_response"
	}
	return []AntigravityPart{{
		FunctionResponse: &AntigravityFunctionResponse{
			ID:   msg.ToolCallId,
			Name: name,
			Response: map[string]any{
				"result": msg.StringContent(),
			},
		},
	}}
}

func antigravityInlineDataFromImage(content dto.MediaContent) *AntigravityInlineData {
	image := content.GetImageMedia()
	if image == nil || strings.TrimSpace(image.Url) == "" || image.IsRemoteImage() {
		return nil
	}
	mimeType, data := splitDataURL(image.Url)
	if data == "" {
		return nil
	}
	if mimeType == "" {
		mimeType = image.MimeType
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	return &AntigravityInlineData{MimeType: mimeType, Data: data}
}

func antigravityInlineDataFromFile(content dto.MediaContent) *AntigravityInlineData {
	file := content.GetFile()
	if file == nil || strings.TrimSpace(file.FileData) == "" {
		return nil
	}
	return &AntigravityInlineData{MimeType: "application/octet-stream", Data: file.FileData}
}

func antigravityInlineDataFromAudio(content dto.MediaContent) *AntigravityInlineData {
	audio := content.GetInputAudio()
	if audio == nil || strings.TrimSpace(audio.Data) == "" {
		return nil
	}
	mimeType := "audio/wav"
	if strings.TrimSpace(audio.Format) != "" {
		mimeType = "audio/" + strings.TrimSpace(audio.Format)
	}
	return &AntigravityInlineData{MimeType: mimeType, Data: audio.Data}
}

func splitDataURL(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "data:") {
		return "", trimmed
	}
	parts := strings.SplitN(strings.TrimPrefix(trimmed, "data:"), ";base64,", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func antigravityGenerationConfig(request *dto.GeneralOpenAIRequest, model string) map[string]any {
	generationConfig := make(map[string]any)
	if strings.Contains(model, "claude") {
		if maxTokens := request.GetMaxTokens(); maxTokens > 0 {
			generationConfig["maxOutputTokens"] = maxTokens
		}
	}
	if request.Temperature != nil {
		generationConfig["temperature"] = *request.Temperature
	}
	if request.TopP != nil {
		generationConfig["topP"] = *request.TopP
	}
	if request.TopK != nil {
		generationConfig["topK"] = *request.TopK
	}
	if request.N != nil && *request.N > 1 {
		generationConfig["candidateCount"] = *request.N
	}
	if effort := strings.ToLower(strings.TrimSpace(request.ReasoningEffort)); effort != "" {
		thinkingConfig := map[string]any{"includeThoughts": effort != "none"}
		if effort == "auto" {
			thinkingConfig["thinkingBudget"] = -1
		} else {
			thinkingConfig["thinkingLevel"] = effort
		}
		generationConfig["thinkingConfig"] = thinkingConfig
	}
	return generationConfig
}

func antigravityTools(tools []dto.ToolCallRequest) []AntigravityTool {
	if len(tools) == 0 {
		return nil
	}
	declarations := make([]AntigravityFunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "" && tool.Type != "function" {
			continue
		}
		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			continue
		}
		declarations = append(declarations, AntigravityFunctionDeclaration{
			Name:        name,
			Description: tool.Function.Description,
			Parameters:  tool.Function.Parameters,
		})
	}
	if len(declarations) == 0 {
		return nil
	}
	return []AntigravityTool{{FunctionDeclarations: declarations}}
}

func antigravityToolConfig(request *dto.GeneralOpenAIRequest, model string, tools []AntigravityTool) *AntigravityToolConfig {
	if len(tools) == 0 {
		return nil
	}
	mode := "AUTO"
	if strings.Contains(model, "claude") {
		mode = "VALIDATED"
	}
	allowedNames := antigravityAllowedFunctionNames(request.ToolChoice)
	return &AntigravityToolConfig{FunctionCallingConfig: AntigravityFunctionCallingConfig{Mode: mode, AllowedFunctionNames: allowedNames}}
}

func antigravityAllowedFunctionNames(toolChoice any) []string {
	choiceMap, ok := toolChoice.(map[string]any)
	if !ok {
		return nil
	}
	functionMap, ok := choiceMap["function"].(map[string]any)
	if !ok {
		return nil
	}
	name, ok := functionMap["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return nil
	}
	return []string{strings.TrimSpace(name)}
}

func generateStableSessionID(request *dto.GeneralOpenAIRequest) string {
	for _, msg := range request.Messages {
		if msg.Role == "user" {
			text := msg.StringContent()
			if text != "" {
				h := sha256.Sum256([]byte(text))
				n := int64(binary.BigEndian.Uint64(h[:8])) & 0x7FFFFFFFFFFFFFFF
				return "-" + strconv.FormatInt(n, 10)
			}
		}
	}
	return generateSessionID()
}

func generateSessionID() string {
	randSourceMutex.Lock()
	n := randSource.Int63n(9_000_000_000_000_000_000)
	randSourceMutex.Unlock()
	return "-" + strconv.FormatInt(n, 10)
}

func generateRequestID() string {
	return "agent-" + uuid.NewString()
}
