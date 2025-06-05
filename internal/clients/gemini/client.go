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
	"unicode/utf8"

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
		textModelName = "gemini-2.5-pro-latest"
		log.Printf("警告：[Gemini Client] 未提供文本分析模型名稱，使用預設值: %s\n", textModelName)
	}
	if videoModelName == "" {
		videoModelName = "gemini-2.5-pro-latest"
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

	return &Client{
		textAnalysisModel:  txtModel,
		videoAnalysisModel: vidModel,
	}, nil
}

// cleanJSONString 清理從 LLM 收到的可能包含雜質的 JSON 字串
func cleanJSONString(rawResponse string) string {
	cleaned := strings.TrimSpace(rawResponse)

	// 移除可能的 markdown 代碼塊標記
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		if strings.HasSuffix(cleaned, "```") {
			cleaned = strings.TrimSuffix(cleaned, "```")
		}
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
		if strings.HasSuffix(cleaned, "```") {
			cleaned = strings.TrimSuffix(cleaned, "```")
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	// 尋找最外層的 JSON 結構
	var potentialJSON string
	firstBrace := strings.Index(cleaned, "{")
	lastBrace := strings.LastIndex(cleaned, "}")
	firstBracket := strings.Index(cleaned, "[")
	lastBracket := strings.LastIndex(cleaned, "]")
	isObject := firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace
	isArray := firstBracket != -1 && lastBracket != -1 && lastBracket > firstBracket

	if isObject && (!isArray || (isArray && firstBrace < firstBracket)) {
		potentialJSON = cleaned[firstBrace : lastBrace+1]
	} else if isArray && (!isObject || (isObject && firstBracket < firstBrace)) {
		potentialJSON = cleaned[firstBracket : lastBracket+1]
	} else {
		potentialJSON = cleaned
	}
	potentialJSON = strings.TrimSpace(potentialJSON)

	// 處理 UTF-8 編碼問題
	if !utf8.ValidString(potentialJSON) {
		log.Println("警告：[Gemini Client Clean] 回應包含無效的 UTF-8 字元，嘗試替換...")
		potentialJSON = strings.ToValidUTF8(potentialJSON, "")
	}

	// 移除控制字元
	var sb strings.Builder
	for _, r := range potentialJSON {
		if (r >= 0 && r < 9) || (r > 10 && r < 13) || (r > 13 && r < 32) || r == 127 {
			continue
		}
		sb.WriteRune(r)
	}
	finalCleaned := sb.String()
	finalCleaned = strings.TrimPrefix(finalCleaned, "\uFEFF")

	// 嘗試解析和重新格式化 JSON
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(finalCleaned), &jsonObj); err != nil {
		log.Printf("警告：[Gemini Client Clean] 初步 JSON 解析失敗，嘗試進一步清理: %v", err)
		// 如果解析失敗，嘗試移除可能的非 JSON 字元
		finalCleaned = strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == '\t' {
				return ' '
			}
			return r
		}, finalCleaned)
		// 移除多餘的空格
		finalCleaned = strings.Join(strings.Fields(finalCleaned), " ")
	} else {
		// 如果解析成功，重新格式化 JSON
		if formattedJSON, err := json.MarshalIndent(jsonObj, "", "  "); err == nil {
			finalCleaned = string(formattedJSON)
		}
	}

	return finalCleaned
}

// AnalyzeText 向 Gemini API 發送純文本內容和提示以進行分析，期望回傳 JSON 字串
func (c *Client) AnalyzeText(ctx context.Context, textContent string, prompt string) (string, error) {
	log.Printf("資訊：[Gemini Client] AnalyzeText - 開始分析文本內容 (長度: %d 字元)\n", len(textContent))
	log.Printf("資訊：[Gemini Client] AnalyzeText - 使用文本分析 Prompt (前100字元): %s...\n", firstNChars(prompt, 100))
	if strings.TrimSpace(textContent) == "" {
		return "", fmt.Errorf("要分析的文本內容不得為空")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("文本分析的 Prompt 不得為空")
	}

	requestParts := []genai.Part{genai.Text(prompt), genai.Text(textContent)}
	log.Println("資訊：[Gemini Client] AnalyzeText - 正在向 Gemini API 發送請求...")
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
			log.Printf("警告：[Gemini Client] AnalyzeText - 收到非預期的 Part 類型: %T\n", part)
		}
	}
	rawJsonResponseString := responseTextBuilder.String()
	if strings.TrimSpace(rawJsonResponseString) == "" {
		return "", fmt.Errorf("Gemini API 文本分析回傳的內容為空")
	}
	log.Printf("資訊：[Gemini Client] AnalyzeText - 收到 API 的原始文字回應 (長度: %d):\nRAW_TEXT_START\n%s\nRAW_TEXT_END\n", len(rawJsonResponseString), rawJsonResponseString)

	cleanedJSONString := cleanJSONString(rawJsonResponseString)
	log.Printf("資訊：[Gemini Client] AnalyzeText - 清理後的 JSON 字串 (長度: %d):\nCLEANED_TEXT_START\n%s\nCLEANED_TEXT_END\n", len(cleanedJSONString), cleanedJSONString)

	if !json.Valid([]byte(cleanedJSONString)) {
		log.Printf("錯誤：[Gemini Client] AnalyzeText - 清理後的字串仍然不是有效的 JSON。完整的 Cleaned JSON String:\n%s\n", cleanedJSONString)
		return "", fmt.Errorf("清理後的字串不是有效的 JSON (文本分析)")
	}
	return cleanedJSONString, nil
}

// AnalyzeVideo 向 Gemini API 發送影片和提示以進行分析
func (c *Client) AnalyzeVideo(ctx context.Context, videoPath string, prompt string) (*models.AnalysisResult, error) {
	log.Printf("資訊：[Gemini Client] AnalyzeVideo - 開始分析影片: %s\n", videoPath)
	log.Printf("資訊：[Gemini Client] AnalyzeVideo - 使用影片分析 Prompt (前100字元): %s...\n", firstNChars(prompt, 100))

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
		log.Printf("警告：[Gemini Client] 未知的影片副檔名 '%s'\n", ext)
	}
	log.Printf("資訊：[Gemini Client] 使用影片 MIME 類型: %s\n", videoMIMEType)
	videoFilePart := genai.Blob{MIMEType: videoMIMEType, Data: videoData}
	requestParts := []genai.Part{genai.Text(prompt), videoFilePart}
	log.Println("資訊：[Gemini Client] AnalyzeVideo - 正在向 Gemini API 發送請求...")
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
			log.Printf("警告：[Gemini Client] AnalyzeVideo - 收到非預期的 Part 類型: %T\n", part)
		}
	}
	rawFullResponseText := responseTextBuilder.String()
	if strings.TrimSpace(rawFullResponseText) == "" {
		return nil, fmt.Errorf("Gemini API 影片分析回傳的文字內容為空")
	}
	log.Printf("資訊：[Gemini Client] AnalyzeVideo - 收到 API 的原始文字回應 (長度: %d):\nRAW_VIDEO_JSON_START\n%s\nRAW_VIDEO_JSON_END\n", len(rawFullResponseText), rawFullResponseText)

	cleanedJSONString := cleanJSONString(rawFullResponseText)
	log.Printf("資訊：[Gemini Client] AnalyzeVideo - 清理後的 JSON 字串準備解析 (長度: %d):\nCLEANED_VIDEO_JSON_START\n%s\nCLEANED_VIDEO_JSON_END\n", len(cleanedJSONString), cleanedJSONString)

	if !json.Valid([]byte(cleanedJSONString)) {
		log.Printf("錯誤：[Gemini Client] AnalyzeVideo - 清理後的字串仍然不是有效的 JSON。完整的 Cleaned JSON String:\n%s\n", cleanedJSONString)
		return nil, fmt.Errorf("清理後的字串不是有效的 JSON (影片分析)")
	}
	var analysis models.AnalysisResult
	if err := json.Unmarshal([]byte(cleanedJSONString), &analysis); err != nil {
		log.Printf("錯誤：[Gemini Client] AnalyzeVideo - 無法將 Gemini API 回應解析為 JSON: %v\n完整的 Cleaned JSON String:\n%s\n", err, cleanedJSONString)
		return nil, fmt.Errorf("無法將 Gemini API 回應解析為 JSON (影片分析): %w。請檢查日誌中的完整 JSON 字串。", err)
	}
	log.Printf("資訊：[Gemini Client] 影片 '%s' JSON 回應解析成功。\n", videoPath)
	return &analysis, nil
}

// firstNChars 輔助函式 - 確保只有一個定義
func firstNChars(s string, n int) string {
	if len(s) > n {
		// 確保不會切割在 UTF-8 字元中間
		// 這是一個簡化的處理，對於非常長的字串和大的 n，效率可能不是最佳
		// 但對於日誌記錄通常足夠
		if n > 0 {
			runes := []rune(s)
			if len(runes) > n {
				return string(runes[:n])
			}
		}
		// 如果 n 較小或轉換為 runes 成本較高，可以回退到簡單的 byte 切割
		// 但要注意可能截斷多位元組字元
		// return s[:n]
	}
	return s
}
