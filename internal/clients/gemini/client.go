package gemini

import (
	"AiHackathon-admin/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Client 結構 (保持不變)
type Client struct {
	genaiModel *genai.GenerativeModel
}

// NewClient (保持不變)
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API Key 不得為空")
	}
	ctx := context.Background()
	genaiClient, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("無法建立 Gemini GenAI 客戶端: %w", err)
	}
	modelName := "gemini-1.5-flash-latest"
	genaiModel := genaiClient.GenerativeModel(modelName)
	// genaiModel.GenerationConfig = &genai.GenerationConfig{
	// 	ResponseMIMEType: "application/json",
	// }
	log.Printf("資訊：Gemini 客戶端初始化成功，使用模型: %s\n", modelName)
	return &Client{genaiModel: genaiModel}, nil
}

// AnalyzeVideo 向 Gemini API 發送影片和提示以進行分析
func (c *Client) AnalyzeVideo(ctx context.Context, videoPath string, prompt string) (*models.AnalysisResult, error) {
	log.Printf("資訊：[Gemini Client] 開始分析影片: %s\n", videoPath)
	log.Printf("資訊：[Gemini Client] 使用 Prompt (前100字元): %s...\n", firstNChars(prompt, 100))

	videoData, err := os.ReadFile(videoPath)
	if err != nil {
		return nil, fmt.Errorf("讀取影片檔案 %s 失敗: %w", videoPath, err)
	}
	videoMIMEType := "video/mp4"
	videoFilePart := genai.Blob{MIMEType: videoMIMEType, Data: videoData}
	requestParts := []genai.Part{genai.Text(prompt), videoFilePart}

	log.Println("資訊：[Gemini Client] 正在向 Gemini API 發送請求...")
	resp, err := c.genaiModel.GenerateContent(ctx, requestParts...)
	if err != nil {
		return nil, fmt.Errorf("Gemini API GenerateContent 失敗: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("Gemini API 回應無效或為空 (nil response or no candidates)")
	}
	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		if candidate.FinishReason != genai.FinishReasonStop && candidate.FinishReason != genai.FinishReasonUnspecified {
			logMsg := fmt.Sprintf("Gemini API 回應無效或內容被阻止，原因: %s", candidate.FinishReason.String())
			if candidate.SafetyRatings != nil {
				for _, rating := range candidate.SafetyRatings {
					log.Printf("警告：[Gemini Client] 安全評級 - Category: %s, Probability: %s\n", rating.Category, rating.Probability)
				}
			}
			return nil, fmt.Errorf(logMsg)
		}
		return nil, fmt.Errorf("Gemini API 回應無效或為空 (no content parts, FinishReason: %s)", candidate.FinishReason.String())
	}

	var responseTextBuilder strings.Builder
	for _, part := range candidate.Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseTextBuilder.WriteString(string(txt))
		}
	}
	fullResponseText := responseTextBuilder.String()
	if strings.TrimSpace(fullResponseText) == "" {
		return nil, fmt.Errorf("Gemini API 回傳的文字內容為空")
	}
	// 為了調試，我們先記錄完整的原始回應，再進行清理
	log.Printf("資訊：[Gemini Client] 收到來自 API 的完整原始文字回應:\n%s\n", fullResponseText)

	var analysis models.AnalysisResult
	cleanedJSONString := strings.TrimSpace(fullResponseText)
	if strings.HasPrefix(cleanedJSONString, "```json") {
		cleanedJSONString = strings.TrimPrefix(cleanedJSONString, "```json")
	}
	if strings.HasSuffix(cleanedJSONString, "```") {
		cleanedJSONString = strings.TrimSuffix(cleanedJSONString, "```")
	}
	cleanedJSONString = strings.TrimSpace(cleanedJSONString)
	log.Printf("資訊：[Gemini Client] 清理後的 JSON 字串準備解析:\n%s\n", cleanedJSONString)

	if err := json.Unmarshal([]byte(cleanedJSONString), &analysis); err != nil {
		// *** 修改日誌記錄，輸出完整的 cleanedJSONString ***
		log.Printf("錯誤：[Gemini Client] 無法將 Gemini API 回應解析為 JSON: %v\n完整的 Cleaned JSON 字串:\n%s\n", err, cleanedJSONString)
		return nil, fmt.Errorf("無法將 Gemini API 回應解析為 JSON: %w。請檢查日誌中的完整 JSON 字串。", err)
	}

	log.Printf("資訊：[Gemini Client] 影片 '%s' JSON 回應解析成功。\n", videoPath)
	return &analysis, nil
}

// firstNChars 輔助函式 (保持不變)
func firstNChars(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
