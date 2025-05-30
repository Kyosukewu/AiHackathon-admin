package gemini

import (
	"AiHackathon-admin/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8" // 新增：用於 UTF-8 驗證和清理

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Client 結構 (保持不變)
type Client struct {
	textAnalysisModel  *genai.GenerativeModel
	videoAnalysisModel *genai.GenerativeModel
}

// NewClient (保持不變)
func NewClient(apiKey string, textModelName string, videoModelName string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API Key 不得為空")
	}
	if textModelName == "" {
		textModelName = "gemini-1.5-flash-latest"
		log.Printf("警告：[Gemini Client] 未提供文本分析模型名稱，使用預設值: %s\n", textModelName)
	}
	if videoModelName == "" {
		videoModelName = "gemini-1.5-flash-latest"
		log.Printf("警告：[Gemini Client] 未提供影片分析模型名稱，使用預設值: %s\n", videoModelName)
	}
	ctx := context.Background()
	genaiSDKClient, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("無法建立 Gemini GenAI SDK 客戶端: %w", err)
	}
	txtModel := genaiSDKClient.GenerativeModel(textModelName)
	var textGenConfig genai.GenerationConfig
	textGenConfig.ResponseMIMEType = "application/json"
	txtModel.GenerationConfig = textGenConfig
	log.Printf("資訊：[Gemini Client] 文本分析模型 '%s' 初始化成功。\n", textModelName)
	vidModel := genaiSDKClient.GenerativeModel(videoModelName)
	var videoGenConfig genai.GenerationConfig
	videoGenConfig.ResponseMIMEType = "application/json"
	vidModel.GenerationConfig = videoGenConfig
	log.Printf("資訊：[Gemini Client] 影片分析模型 '%s' 初始化成功。\n", videoModelName)
	return &Client{textAnalysisModel: txtModel, videoAnalysisModel: vidModel}, nil
}

// cleanAndValidateJSON 清理並驗證 JSON 字串
func cleanAndValidateJSON(rawJSON string, taskType string) (string, error) {
	cleaned := strings.TrimSpace(rawJSON)
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
	}
	if strings.HasSuffix(cleaned, "```") {
		cleaned = strings.TrimSuffix(cleaned, "```")
	}
	cleaned = strings.TrimSpace(cleaned)

	if !utf8.ValidString(cleaned) {
		log.Printf("警告：[Gemini Client - %s] 回應包含無效的 UTF-8 字元，嘗試清理...", taskType)
		cleaned = strings.ToValidUTF8(cleaned, "�") // 將無效字元替換為 � (U+FFFD)
	}

	// 確保至少是基本的 JSON 結構 (以 { 或 [ 開頭，以 } 或 ] 結尾)
	if !((strings.HasPrefix(cleaned, "{") && strings.HasSuffix(cleaned, "}")) ||
		(strings.HasPrefix(cleaned, "[") && strings.HasSuffix(cleaned, "]"))) {
		log.Printf("錯誤：[Gemini Client - %s] 清理後的回應不是以 { 或 [ 開頭並以 } 或 ] 結尾的有效 JSON 結構。內容:\n%s\n", taskType, cleaned)
		return "", fmt.Errorf("清理後的回應不是有效的 JSON 結構 (缺少正確的括號)")
	}

	// 嘗試驗證 JSON 結構是否有效 (僅檢查語法，不檢查 schema)
	var js json.RawMessage
	if err := json.Unmarshal([]byte(cleaned), &js); err != nil {
		log.Printf("錯誤：[Gemini Client - %s] 清理後的回應無法通過初步 JSON 驗證: %v。內容:\n%s\n", taskType, err, cleaned)
		return "", fmt.Errorf("清理後的回應不是有效的 JSON: %w", err)
	}

	return cleaned, nil
}

// AnalyzeText 向 Gemini API 發送純文本內容和提示以進行分析，期望回傳 JSON 字串
func (c *Client) AnalyzeText(ctx context.Context, textContent string, prompt string) (string, error) {
	log.Printf("資訊：[Gemini Client] 開始分析文本內容 (長度: %d 字元)\n", len(textContent))
	log.Printf("資訊：[Gemini Client] 使用文本分析 Prompt (前100字元): %s...\n", firstNChars(prompt, 100))
	if strings.TrimSpace(textContent) == "" {
		return "", fmt.Errorf("要分析的文本內容不得為空")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("文本分析的 Prompt 不得為空")
	}
	requestParts := []genai.Part{genai.Text(prompt), genai.Text(textContent)}
	log.Println("資訊：[Gemini Client] 正在向 Gemini API 發送文本分析請求...")
	resp, err := c.textAnalysisModel.GenerateContent(ctx, requestParts...)
	if err != nil {
		return "", fmt.Errorf("Gemini API 文本分析 GenerateContent 失敗: %w", err)
	}
	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("Gemini API 文本分析回應無效或為空 (nil response or no candidates)")
	}
	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		if candidate.FinishReason != genai.FinishReasonStop && candidate.FinishReason != genai.FinishReasonUnspecified {
			logMsg := fmt.Sprintf("Gemini API 文本分析回應無效或內容被阻止，原因: %s", candidate.FinishReason.String())
			if candidate.SafetyRatings != nil {
				for _, rating := range candidate.SafetyRatings {
					log.Printf("警告：[Gemini Client] 安全評級 (文本分析) - Category: %s, Probability: %s\n", rating.Category, rating.Probability)
				}
			}
			return "", fmt.Errorf(logMsg)
		}
		return "", fmt.Errorf("Gemini API 文本分析回應無效或為空 (no content parts, FinishReason: %s)", candidate.FinishReason.String())
	}
	var responseTextBuilder strings.Builder
	for _, part := range candidate.Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseTextBuilder.WriteString(string(txt))
		} else {
			log.Printf("警告：[Gemini Client] 文本分析收到非預期的 Part 類型: %T\n", part)
		}
	}
	rawJsonResponseString := responseTextBuilder.String()
	if strings.TrimSpace(rawJsonResponseString) == "" {
		return "", fmt.Errorf("Gemini API 文本分析回傳的 JSON 內容為空")
	}

	cleanedJSON, err := cleanAndValidateJSON(rawJsonResponseString, "TextAnalysis")
	if err != nil {
		log.Printf("錯誤：[Gemini Client] 文本分析回應清理或驗證失敗: %v\n原始回應:\n%s\n", err, rawJsonResponseString)
		return "", fmt.Errorf("文本分析回應清理或驗證失敗: %w", err)
	}
	log.Printf("資訊：[Gemini Client] 收到並清理文本分析 API 的 JSON 回應:\n%s\n", cleanedJSON)
	return cleanedJSON, nil
}

// AnalyzeVideo 向 Gemini API 發送影片和提示以進行分析
func (c *Client) AnalyzeVideo(ctx context.Context, videoPath string, prompt string) (*models.AnalysisResult, error) {
	log.Printf("資訊：[Gemini Client] 開始分析影片: %s\n", videoPath)
	log.Printf("資訊：[Gemini Client] 使用影片分析 Prompt (前100字元): %s...\n", firstNChars(prompt, 100))
	videoData, err := os.ReadFile(videoPath)
	if err != nil {
		return nil, fmt.Errorf("讀取影片檔案 %s 失敗: %w", videoPath, err)
	}
	videoMIMEType := "video/mp4"
	ext := strings.ToLower(filepath.Ext(videoPath))
	switch ext {
	case ".mp4":
		videoMIMEType = "video/mp4"
	case ".mov":
		videoMIMEType = "video/quicktime"
	case ".mpeg", ".mpg":
		videoMIMEType = "video/mpeg"
	case ".avi":
		videoMIMEType = "video/x-msvideo"
	case ".wmv":
		videoMIMEType = "video/x-ms-wmv"
	case ".flv":
		videoMIMEType = "video/x-flv"
	case ".webm":
		videoMIMEType = "video/webm"
	default:
		log.Printf("警告：[Gemini Client] 未知的影片副檔名 '%s'，將使用預設 MIME 類型 '%s'\n", ext, videoMIMEType)
	}
	log.Printf("資訊：[Gemini Client] 使用影片 MIME 類型: %s\n", videoMIMEType)
	videoFilePart := genai.Blob{MIMEType: videoMIMEType, Data: videoData}
	requestParts := []genai.Part{genai.Text(prompt), videoFilePart}
	log.Println("資訊：[Gemini Client] 正在向 Gemini API 發送影片分析請求...")
	resp, err := c.videoAnalysisModel.GenerateContent(ctx, requestParts...)
	if err != nil {
		return nil, fmt.Errorf("Gemini API 影片分析 GenerateContent 失敗: %w", err)
	}
	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("Gemini API 影片分析回應無效或為空 (nil response or no candidates)")
	}
	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		if candidate.FinishReason != genai.FinishReasonStop && candidate.FinishReason != genai.FinishReasonUnspecified {
			logMsg := fmt.Sprintf("Gemini API 影片分析回應無效或內容被阻止，原因: %s", candidate.FinishReason.String())
			if candidate.SafetyRatings != nil {
				for _, rating := range candidate.SafetyRatings {
					log.Printf("警告：[Gemini Client] 安全評級 (影片分析) - Category: %s, Probability: %s\n", rating.Category, rating.Probability)
				}
			}
			return nil, fmt.Errorf(logMsg)
		}
		return nil, fmt.Errorf("Gemini API 影片分析回應無效或為空 (no content parts, FinishReason: %s)", candidate.FinishReason.String())
	}
	var responseTextBuilder strings.Builder
	for _, part := range candidate.Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseTextBuilder.WriteString(string(txt))
		} else {
			log.Printf("警告：[Gemini Client] 影片分析收到非預期的 Part 類型: %T\n", part)
		}
	}
	rawFullResponseText := responseTextBuilder.String()
	if strings.TrimSpace(rawFullResponseText) == "" {
		return nil, fmt.Errorf("Gemini API 影片分析回傳的文字內容為空")
	}

	cleanedJSONString, err := cleanAndValidateJSON(rawFullResponseText, "VideoAnalysis")
	if err != nil {
		log.Printf("錯誤：[Gemini Client] 影片分析回應清理或驗證失敗: %v\n原始回應:\n%s\n", err, rawFullResponseText)
		return nil, fmt.Errorf("影片分析回應清理或驗證失敗: %w", err)
	}
	log.Printf("資訊：[Gemini Client] 清理後的 JSON 字串準備解析 (影片分析):\n%s\n", cleanedJSONString)

	var analysis models.AnalysisResult
	if err := json.Unmarshal([]byte(cleanedJSONString), &analysis); err != nil {
		log.Printf("錯誤：[Gemini Client] 無法將 Gemini API 回應解析為 JSON (影片分析): %v\n完整的 Cleaned JSON 字串:\n%s\n", err, cleanedJSONString)
		return nil, fmt.Errorf("無法將 Gemini API 回應解析為 JSON (影片分析): %w。請檢查日誌中的完整 JSON 字串。", err)
	}
	log.Printf("資訊：[Gemini Client] 影片 '%s' JSON 回應解析成功。\n", videoPath)
	return &analysis, nil
}

// firstNChars (保持不變)
func firstNChars(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
