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

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Client 結構用於與 Gemini API 互動
type Client struct {
	textAnalysisModel  *genai.GenerativeModel
	videoAnalysisModel *genai.GenerativeModel
}

// NewClient 建立一個 Gemini 客戶端實例
func NewClient(apiKey string, textModelName string, videoModelName string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API Key 不得為空")
	}
	if textModelName == "" {
		textModelName = "gemini-1.5-flash-latest" // 預設文本模型
		log.Printf("警告：[Gemini Client] 未提供文本分析模型名稱，使用預設值: %s\n", textModelName)
	}
	if videoModelName == "" {
		videoModelName = "gemini-1.5-flash-latest" // 預設影片模型 (多模態)
		log.Printf("警告：[Gemini Client] 未提供影片分析模型名稱，使用預設值: %s\n", videoModelName)
	}

	ctx := context.Background()
	genaiSDKClient, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("無法建立 Gemini GenAI SDK 客戶端: %w", err)
	}

	// 初始化文本分析模型
	txtModel := genaiSDKClient.GenerativeModel(textModelName)
	// --- 根據編譯器錯誤，假設 GenerationConfig 是值類型 ---
	// 創建一個 genai.GenerationConfig 的值
	var textGenConfig genai.GenerationConfig
	textGenConfig.ResponseMIMEType = "application/json"
	// 如果需要設定 Temperature (假設 Temperature 是 *float32)
	// tempVal := float32(0.7)
	// textGenConfig.Temperature = &tempVal
	txtModel.GenerationConfig = textGenConfig // 賦值 (值類型)
	// --- 結束假設 ---
	log.Printf("資訊：[Gemini Client] 文本分析模型 '%s' 初始化成功。\n", textModelName)

	// 初始化影片分析模型
	vidModel := genaiSDKClient.GenerativeModel(videoModelName)
	// --- 根據編譯器錯誤，假設 GenerationConfig 是值類型 ---
	var videoGenConfig genai.GenerationConfig
	videoGenConfig.ResponseMIMEType = "application/json"
	// videoGenConfig.MaxOutputTokens = // 如果 MaxOutputTokens 是 *int32
	// maxTokens := int32(8192)
	// videoGenConfig.MaxOutputTokens = &maxTokens
	vidModel.GenerationConfig = videoGenConfig // 賦值 (值類型)
	// --- 結束假設 ---
	log.Printf("資訊：[Gemini Client] 影片分析模型 '%s' 初始化成功。\n", videoModelName)

	return &Client{
		textAnalysisModel:  txtModel,
		videoAnalysisModel: vidModel,
	}, nil
}

// AnalyzeText (與之前版本相同)
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
	jsonResponseString := responseTextBuilder.String()
	if strings.TrimSpace(jsonResponseString) == "" {
		return "", fmt.Errorf("Gemini API 文本分析回傳的 JSON 內容為空")
	}
	log.Printf("資訊：[Gemini Client] 收到文本分析 API 的完整 JSON 回應:\n%s\n", jsonResponseString)
	return jsonResponseString, nil
}

// AnalyzeVideo (與之前版本相同)
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
	fullResponseText := responseTextBuilder.String()
	if strings.TrimSpace(fullResponseText) == "" {
		return nil, fmt.Errorf("Gemini API 影片分析回傳的文字內容為空")
	}
	log.Printf("資訊：[Gemini Client] 收到 API 的完整文字回應 (影片分析) (前500字元): %s...\n", firstNChars(fullResponseText, 500))
	var analysis models.AnalysisResult
	cleanedJSONString := strings.TrimSpace(fullResponseText)
	if strings.HasPrefix(cleanedJSONString, "```json") {
		cleanedJSONString = strings.TrimPrefix(cleanedJSONString, "```json")
	}
	if strings.HasSuffix(cleanedJSONString, "```") {
		cleanedJSONString = strings.TrimSuffix(cleanedJSONString, "```")
	}
	cleanedJSONString = strings.TrimSpace(cleanedJSONString)
	log.Printf("資訊：[Gemini Client] 清理後的 JSON 字串準備解析 (影片分析):\n%s\n", cleanedJSONString)
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
