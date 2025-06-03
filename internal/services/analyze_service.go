package services

import (
	"AiHackathon-admin/internal/clients/gemini"
	"AiHackathon-admin/internal/config"
	"AiHackathon-admin/internal/models"
	"AiHackathon-admin/internal/web/handlers"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// NASStorage 介面定義了 AnalyzeService 需要的 NAS 操作。
// (應在 internal/services/interfaces.go 中唯一定義)
// type NASStorage interface {
// 	ReadVideo(filePath string) ([]byte, error)
// }

// AnalyzeService 結構
type AnalyzeService struct {
	cfg          *config.Config
	db           handlers.DBStore
	nas          NASStorage // 來自同 package services 下的 interfaces.go
	geminiClient *gemini.Client
}

// NewAnalyzeService 建立 AnalyzeService 實例
func NewAnalyzeService(
	cfg *config.Config,
	db handlers.DBStore,
	nas NASStorage,
	geminiClient *gemini.Client,
) (*AnalyzeService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("AnalyzeService：設定不得為空")
	}
	if db == nil {
		return nil, fmt.Errorf("AnalyzeService：DBStore 不得為空")
	}
	if nas == nil {
		return nil, fmt.Errorf("AnalyzeService：NASStorage 不得為空")
	}
	if geminiClient == nil {
		return nil, fmt.Errorf("AnalyzeService：Gemini 客戶端不得為空")
	}
	log.Println("資訊：AnalyzeService 初始化完成。")
	return &AnalyzeService{
		cfg:          cfg,
		db:           db,
		nas:          nas,
		geminiClient: geminiClient,
	}, nil
}

// scanVideoFiles 掃描 NAS 路徑，找到成對的影片檔案和 .txt 描述檔
func (s *AnalyzeService) scanVideoFiles() ([]models.VideoFileInfo, error) {
	var videoFileInfos []models.VideoFileInfo
	nasBasePath, err := filepath.Abs(s.cfg.NAS.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("無法取得 NAS videoPath 的絕對路徑 '%s': %w", s.cfg.NAS.VideoPath, err)
	}
	log.Printf("資訊：[AnalyzeService] 開始掃描影片及 TXT 檔案於路徑: %s (新結構)\n", nasBasePath)
	supportedVideoExtensions := map[string]bool{
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".ts": true, ".flv": true, ".wmv": true,
	}
	txtExtension := ".txt"
	sourceDirs, err := os.ReadDir(nasBasePath)
	if err != nil {
		return nil, fmt.Errorf("[AnalyzeService] 讀取 NAS 根目錄 '%s' 失敗: %w", nasBasePath, err)
	}
	for _, sourceDirEntry := range sourceDirs {
		if !sourceDirEntry.IsDir() {
			continue
		}
		sourceName := sourceDirEntry.Name()
		sourcePath := filepath.Join(nasBasePath, sourceName)
		videoIDDirs, err := os.ReadDir(sourcePath)
		if err != nil {
			log.Printf("警告：[AnalyzeService] 讀取來源目錄 '%s' 失敗: %v\n", sourcePath, err)
			continue
		}
		for _, videoIDDirEntry := range videoIDDirs {
			if !videoIDDirEntry.IsDir() {
				continue
			}
			videoID := videoIDDirEntry.Name()
			videoIDPath := filepath.Join(sourcePath, videoID)
			var videoFilePath, txtFilePath, videoFileName string
			var modTime time.Time
			entriesInVideoIDDir, err := os.ReadDir(videoIDPath)
			if err != nil {
				log.Printf("警告：[AnalyzeService] 讀取影片ID目錄 '%s' 失敗: %v\n", videoIDPath, err)
				continue
			}
			for _, entry := range entriesInVideoIDDir {
				if entry.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if supportedVideoExtensions[ext] {
					if videoFilePath == "" {
						videoFilePath = filepath.Join(videoIDPath, entry.Name())
						videoFileName = entry.Name()
						fi, _ := entry.Info()
						if fi != nil {
							modTime = fi.ModTime()
						}
					} else {
						log.Printf("警告：[AnalyzeService] 影片ID目錄 '%s' 中找到多個影片檔案，使用第一個: %s\n", videoIDPath, videoFileName)
					}
				} else if ext == txtExtension {
					if txtFilePath == "" {
						txtFilePath = filepath.Join(videoIDPath, entry.Name())
					} else {
						log.Printf("警告：[AnalyzeService] 影片ID目錄 '%s' 中找到多個 TXT 檔案，使用第一個。\n", videoIDPath)
					}
				}
			}
			if videoFilePath != "" && txtFilePath != "" {
				relativePath, _ := filepath.Rel(nasBasePath, videoFilePath)
				videoFileInfos = append(videoFileInfos, models.VideoFileInfo{VideoAbsolutePath: videoFilePath, TextFilePath: txtFilePath, RelativePath: relativePath, SourceName: sourceName, OriginalID: videoID, VideoFileName: videoFileName, ModTime: modTime})
				log.Printf("資訊：[AnalyzeService] 找到匹配的影片和TXT: V: %s, T: %s (來源: %s, ID: %s)\n", videoFileName, filepath.Base(txtFilePath), sourceName, videoID)
			} else {
				if videoFilePath == "" && txtFilePath != "" {
					log.Printf("警告：[AnalyzeService] 影片ID目錄 '%s' 中只找到 TXT 檔案。\n", videoIDPath)
				} else if videoFilePath != "" && txtFilePath == "" {
					log.Printf("警告：[AnalyzeService] 影片ID目錄 '%s' 中只找到影片檔案。\n", videoIDPath)
				}
			}
		}
	}
	log.Printf("資訊：[AnalyzeService] 掃描完成，共找到 %d 組有效的影片/TXT 配對。\n", len(videoFileInfos))
	return videoFileInfos, nil
}

// *** getPromptTextAndVersionFromFile 函式定義 ***
func getPromptTextAndVersionFromFile(configuredPath, versionKeyFromConfig, fallbackPrompt, fallbackVersionKey string) (string, string, error) {
	if configuredPath == "" {
		log.Printf("警告：[Service] Prompt 版本 '%s' 的檔案路徑為空，將使用備用 Prompt (版本: %s)。\n", versionKeyFromConfig, fallbackVersionKey)
		return fallbackPrompt, fallbackVersionKey, nil
	}
	if _, err := os.Stat(configuredPath); os.IsNotExist(err) {
		log.Printf("錯誤：[Service] Prompt 檔案 '%s' (版本 '%s') 不存在，將使用備用 Prompt (版本: %s)。\n", configuredPath, versionKeyFromConfig, fallbackVersionKey)
		return fallbackPrompt, fallbackVersionKey, fmt.Errorf("prompt 檔案 '%s' (版本 '%s') 不存在", configuredPath, versionKeyFromConfig)
	}
	promptBytes, err := os.ReadFile(configuredPath)
	if err != nil {
		log.Printf("錯誤：[Service] 讀取 Prompt 檔案 '%s' (版本 '%s') 失敗: %v。將使用備用 Prompt (版本: %s)。\n", configuredPath, versionKeyFromConfig, err, fallbackVersionKey)
		return fallbackPrompt, fallbackVersionKey, fmt.Errorf("讀取 prompt 檔案 '%s' (版本 '%s') 失敗: %w", configuredPath, versionKeyFromConfig, err)
	}
	return string(promptBytes), versionKeyFromConfig, nil // 成功讀取檔案，使用配置的版本號
}

// *** 結束 getPromptTextAndVersionFromFile 定義 ***

// analyzeTextFileContent 使用 Gemini 分析 TXT 檔案內容並回傳結構化的元數據
func (s *AnalyzeService) analyzeTextFileContent(ctx context.Context, txtFilePath string) (*models.ParsedTxtData, string, error) {
	log.Printf("資訊：[AnalyzeService] 開始使用 Gemini 分析 TXT 檔案: %s\n", txtFilePath)
	txtContentBytes, err := os.ReadFile(txtFilePath)
	if err != nil {
		return nil, "", fmt.Errorf("無法讀取 TXT 檔案 '%s': %w", txtFilePath, err)
	}
	txtContent := string(txtContentBytes)
	if strings.TrimSpace(txtContent) == "" {
		log.Printf("警告：[AnalyzeService] TXT 檔案 '%s' 內容為空，跳過 Gemini 分析。\n", txtFilePath)
		return &models.ParsedTxtData{}, "no_prompt_needed_empty_txt", nil
	}

	currentVersionKey := s.cfg.Prompts.TextFileAnalysis.CurrentVersion
	promptFilePath, pathOk := s.cfg.Prompts.TextFileAnalysis.Versions[currentVersionKey] // pathOk 在這裡被賦值
	if !pathOk {                                                                         // *** 使用 pathOk 檢查 ***
		log.Printf("警告：[AnalyzeService] TextFileAnalysis Prompt 版本 '%s' 在設定檔的 versions map 中未找到對應路徑。", currentVersionKey)
		return nil, currentVersionKey, fmt.Errorf("未在 versions map 中找到文本分析 Prompt 的檔案路徑 (版本: %s)", currentVersionKey)
	}
	if promptFilePath == "" { // 額外檢查路徑是否為空
		log.Printf("警告：[AnalyzeService] TextFileAnalysis Prompt 版本 '%s' 的檔案路徑為空。", currentVersionKey)
		return nil, currentVersionKey, fmt.Errorf("文本分析 Prompt 版本 '%s' 的檔案路徑為空", currentVersionKey)
	}

	fallbackPrompt := "請從提供的文本中提取標題、創建日期（YYYY-MM-DD HH:MM:SS）、時長（秒）、主題（陣列）、地點和 SHOTLIST，並以 JSON 格式回傳。"
	promptText, actualPromptVersion, errPrompt := getPromptTextAndVersionFromFile(promptFilePath, currentVersionKey, fallbackPrompt, "default-text-fallback")
	if errPrompt != nil && promptFilePath != "" {
		log.Printf("警告：讀取指定的文本 Prompt 檔案 '%s' 失敗 (%v)，將使用硬編碼的備用 Prompt。", promptFilePath, errPrompt)
	}
	log.Printf("資訊：[AnalyzeService] 使用 TextFileAnalysis Prompt 版本: %s (來自檔案: %s)\n", actualPromptVersion, promptFilePath)

	cleanedJSONString, err := s.geminiClient.AnalyzeText(ctx, txtContent, promptText)
	if err != nil {
		return nil, actualPromptVersion, fmt.Errorf("Gemini 分析 TXT 內容失敗 ('%s'): %w", txtFilePath, err)
	}

	if cleanedJSONString == "" {
		log.Printf("警告：[AnalyzeService] Gemini 對 TXT 檔案 '%s' 的分析回傳了空的或無效的 JSON 字串。\n", txtFilePath)
		return &models.ParsedTxtData{}, actualPromptVersion, nil
	}

	var parsedData models.ParsedTxtData // *** parsedData 在這裡宣告 ***
	if err := json.Unmarshal([]byte(cleanedJSONString), &parsedData); err != nil {
		log.Printf("錯誤：[AnalyzeService] 無法將 TXT 分析回應解析為 JSON: %v\nCleaned JSON String WAS:\n%s\n", err, cleanedJSONString)
		return nil, actualPromptVersion, fmt.Errorf("無法將 TXT 分析回應解析為 JSON (cleaned): %w。查看日誌中的完整 JSON。", err)
	}

	log.Printf("資訊：[AnalyzeService] TXT 檔案 '%s' Gemini 分析並解析 JSON 成功。\n", txtFilePath)
	return &parsedData, actualPromptVersion, nil
}

// buildPromptForVideo (修正 ok 的使用)
func (s *AnalyzeService) buildPromptForVideo(videoInfo models.VideoFileInfo, txtAnalyzedData *models.ParsedTxtData) (promptText string, promptVersion string) {
	currentVersionKey := s.cfg.Prompts.VideoAnalysis.CurrentVersion
	promptFilePath, pathOk := s.cfg.Prompts.VideoAnalysis.Versions[currentVersionKey] // pathOk 在這裡被賦值
	fallbackPrompt := "請分析此影片的音視覺內容，提供短摘要、列點摘要、BITE、影片中提及的地點、重要性評分、關鍵詞、影片內容的分類和素材類型。"

	if !pathOk { // *** 使用 pathOk 檢查 ***
		log.Printf("警告：[AnalyzeService] VideoAnalysis Prompt 版本 '%s' 在設定檔的 versions map 中未找到對應路徑。將使用備用。", currentVersionKey)
		return fallbackPrompt, "default-video-fallback-no-key"
	}
	if promptFilePath == "" { // 額外檢查路徑是否為空
		log.Printf("警告：[AnalyzeService] VideoAnalysis Prompt 版本 '%s' 的檔案路徑為空。將使用備用。", currentVersionKey)
		return fallbackPrompt, "default-video-fallback-empty-path"
	}

	textFromFile, actualPromptVersion, errPrompt := getPromptTextAndVersionFromFile(promptFilePath, currentVersionKey, fallbackPrompt, "default-video-fallback-read-err")
	if errPrompt != nil && promptFilePath != "" {
		log.Printf("警告：讀取指定的影片 Prompt 檔案 '%s' 失敗 (%v)，將使用硬編碼的備用 Prompt。", promptFilePath, errPrompt)
	}
	log.Printf("資訊：[AnalyzeService] 使用 VideoAnalysis Prompt 版本: %s (來自檔案: %s)\n", actualPromptVersion, promptFilePath)
	return textFromFile, actualPromptVersion
}

// logAnalysisResult (保持不變)
func (s *AnalyzeService) logAnalysisResult(videoPath string, result *models.AnalysisResult) { /* ... */
}

// ExecuteTextAnalysisPipeline (保持不變)
func (s *AnalyzeService) ExecuteTextAnalysisPipeline() error { /* ...與您之前的版本相同... */
	log.Println("資訊：[AnalyzeService-TextPipeline] 開始執行文本元數據分析流程...")
	videoFileInfos, err := s.scanVideoFiles()
	if err != nil {
		log.Printf("錯誤：[AnalyzeService-TextPipeline] 掃描檔案失敗: %v", err)
		return err
	}
	if len(videoFileInfos) == 0 {
		log.Println("資訊：[AnalyzeService-TextPipeline] 未找到任何影片/TXT 配對進行處理。")
		return nil
	}
	var successCount, failCount int
	for _, videoInfo := range videoFileInfos {
		log.Printf("資訊：[AnalyzeService-TextPipeline] 處理 TXT 檔案: %s (影片: %s)\n", videoInfo.TextFilePath, videoInfo.VideoFileName)
		baseVideoForFind := &models.Video{SourceName: videoInfo.SourceName, SourceID: videoInfo.OriginalID, NASPath: videoInfo.RelativePath, FetchedAt: videoInfo.ModTime}
		videoID, findErr := s.db.FindOrCreateVideo(baseVideoForFind)
		if findErr != nil {
			log.Printf("錯誤：[AnalyzeService-TextPipeline] 為 TXT '%s' 查找/建立基礎影片記錄失敗: %v", videoInfo.TextFilePath, findErr)
			failCount++
			continue
		}
		if videoID == 0 {
			log.Printf("錯誤：[AnalyzeService-TextPipeline] FindOrCreateVideo 為 TXT '%s' 回傳了無效的 videoID (0)。\n", videoInfo.TextFilePath)
			failCount++
			continue
		}
		existingVideo, getErr := s.db.GetVideoByID(videoID)
		if getErr != nil {
			log.Printf("錯誤：[AnalyzeService-TextPipeline] 查詢影片 ID %d 狀態失敗: %v. 跳過此文本分析.\n", videoID, getErr)
			failCount++
			continue
		}
		if existingVideo != nil && (existingVideo.AnalysisStatus == models.StatusMetadataExtracted || existingVideo.AnalysisStatus == models.StatusProcessing || existingVideo.AnalysisStatus == models.StatusCompleted || existingVideo.AnalysisStatus == models.StatusVideoAnalysisFailed) {
			log.Printf("資訊：[AnalyzeService-TextPipeline] 影片 ID %d (TXT: %s) 狀態為 %s，已提取過元數據或正在/已完成後續分析，跳過文本分析。\n", videoID, videoInfo.TextFilePath, existingVideo.AnalysisStatus)
			continue
		}
		updateStatusErr := s.db.UpdateVideoAnalysisStatus(videoID, models.StatusMetadataExtracting, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{})
		if updateStatusErr != nil {
			log.Printf("警告：[AnalyzeService-TextPipeline] 更新影片 ID %d 狀態為 '%s' 失敗: %v\n", videoID, models.StatusMetadataExtracting, updateStatusErr)
		}
		ctxTxt, cancelTxt := context.WithTimeout(context.Background(), 3*time.Minute)
		parsedTxtData, _, txtErr := s.analyzeTextFileContent(ctxTxt, videoInfo.TextFilePath)
		cancelTxt()
		currentTime := time.Now()
		if txtErr != nil {
			log.Printf("錯誤：[AnalyzeService-TextPipeline] 分析 TXT 檔案 '%s' (VideoID: %d) 失敗: %v\n", videoInfo.TextFilePath, videoID, txtErr)
			s.db.UpdateVideoAnalysisStatus(videoID, models.StatusTxtAnalysisFailed, sql.NullTime{Time: currentTime, Valid: true}, sql.NullString{String: "TXT分析失敗: " + txtErr.Error(), Valid: true})
			failCount++
			continue
		}
		if parsedTxtData == nil {
			log.Printf("錯誤：[AnalyzeService-TextPipeline] analyzeTextFileContent 為 TXT '%s' 回傳了 nil parsedTxtData 但沒有錯誤。", videoInfo.TextFilePath)
			s.db.UpdateVideoAnalysisStatus(videoID, models.StatusTxtAnalysisFailed, sql.NullTime{Time: currentTime, Valid: true}, sql.NullString{String: "TXT分析回傳nil數據", Valid: true})
			failCount++
			continue
		}
		videoToUpdate := &models.Video{ID: videoID, SourceName: videoInfo.SourceName, SourceID: videoInfo.OriginalID, NASPath: videoInfo.RelativePath, FetchedAt: existingVideo.FetchedAt, Title: sql.NullString{String: parsedTxtData.Title, Valid: parsedTxtData.Title != ""}, ShotlistContent: models.JsonNullString{NullString: sql.NullString{String: parsedTxtData.ShotlistContent, Valid: parsedTxtData.ShotlistContent != ""}}, Location: sql.NullString{String: parsedTxtData.Location, Valid: parsedTxtData.Location != ""}, Subjects: parsedTxtData.Subjects, AnalysisStatus: models.StatusMetadataExtracted, AnalyzedAt: sql.NullTime{Time: currentTime, Valid: true}, ViewLink: existingVideo.ViewLink, SourceMetadata: existingVideo.SourceMetadata}
		if !videoInfo.ModTime.IsZero() && videoInfo.ModTime.After(existingVideo.FetchedAt) {
			videoToUpdate.FetchedAt = videoInfo.ModTime
		}
		if parsedTxtData.CreationDateStr != "" {
			parsedTime, errDate := time.Parse("2006-01-02 15:04:05", parsedTxtData.CreationDateStr)
			if errDate == nil {
				videoToUpdate.PublishedAt = sql.NullTime{Time: parsedTime, Valid: true}
			} else {
				log.Printf("警告：[AnalyzeService-TextPipeline] 無法解析 TXT CreationDate '%s': %v", parsedTxtData.CreationDateStr, errDate)
				videoToUpdate.PublishedAt = existingVideo.PublishedAt
			}
		} else {
			videoToUpdate.PublishedAt = existingVideo.PublishedAt
		}
		if len(parsedTxtData.DurationSeconds) > 0 && string(parsedTxtData.DurationSeconds) != "null" {
			var durationInt int
			var durationStr string
			rawDurationContent := string(parsedTxtData.DurationSeconds)
			if err := json.Unmarshal(parsedTxtData.DurationSeconds, &durationInt); err == nil {
				if durationInt > 0 {
					videoToUpdate.DurationSecs = sql.NullInt64{Int64: int64(durationInt), Valid: true}
				}
			} else if err := json.Unmarshal(parsedTxtData.DurationSeconds, &durationStr); err == nil {
				durationIntConv, convErr := strconv.Atoi(durationStr)
				if convErr == nil && durationIntConv > 0 {
					videoToUpdate.DurationSecs = sql.NullInt64{Int64: int64(durationIntConv), Valid: true}
				} else {
					log.Printf("警告：[AnalyzeService-TextPipeline] 無法將 TXT DurationSeconds 字串 '%s' (來自JSON字串 '%s') 解析為數字: %v", durationStr, rawDurationContent, convErr)
					videoToUpdate.DurationSecs = existingVideo.DurationSecs
				}
			} else {
				log.Printf("警告：[AnalyzeService-TextPipeline] TXT DurationSeconds ('%s') 解析為數字或字串均失敗。", rawDurationContent)
				videoToUpdate.DurationSecs = existingVideo.DurationSecs
			}
		} else {
			videoToUpdate.DurationSecs = existingVideo.DurationSecs
		}
		_, dbErr := s.db.FindOrCreateVideo(videoToUpdate)
		if dbErr != nil {
			log.Printf("錯誤：[AnalyzeService-TextPipeline] 更新影片 '%s' 的 TXT 元數據到資料庫失敗: %v\n", videoInfo.RelativePath, dbErr)
			failCount++
			continue
		}
		log.Printf("資訊：[AnalyzeService-TextPipeline] TXT 元數據已為影片 ID %d 更新/儲存。\n", videoID)
		successCount++
	}
	log.Printf("資訊：[AnalyzeService-TextPipeline] 文本元數據分析流程完成。成功: %d, 失敗: %d\n", successCount, failCount)
	return nil
}

func (s *AnalyzeService) ExecuteVideoContentPipeline() error {
	log.Println("資訊：[AnalyzeService-VideoPipeline] ExecuteVideoContentPipeline 方法已進入。")

	statusToQuery := models.StatusMetadataExtracted
	log.Printf("資訊：[AnalyzeService-VideoPipeline] 準備從資料庫查詢狀態為 '%s' 的影片...\n", statusToQuery)

	videosToAnalyze, err := s.db.GetVideosPendingContentAnalysis(statusToQuery, 10)

	// *** 新增詳細日誌 ***
	if err != nil {
		log.Printf("錯誤：[AnalyzeService-VideoPipeline] 呼叫 GetVideosPendingContentAnalysis 時發生錯誤: %v", err)
		return err
	}
	log.Printf("DEBUG：[AnalyzeService-VideoPipeline] GetVideosPendingContentAnalysis 回傳了 %d 個影片。錯誤（如果有的話）: %v\n", len(videosToAnalyze), err)
	// *** 結束新增日誌 ***

	if len(videosToAnalyze) == 0 {
		log.Println("資訊：[AnalyzeService-VideoPipeline] 資料庫中沒有找到狀態為 'metadata_extracted' 的影片進行內容分析。流程結束。")
		return nil
	}

	log.Printf("資訊：[AnalyzeService-VideoPipeline] 找到 %d 個影片準備進行內容分析。\n", len(videosToAnalyze))

	var successCount, failCount int
	for _, video := range videosToAnalyze {
		log.Printf("資訊：[AnalyzeService-VideoPipeline] 開始處理影片內容分析: %s (ID: %d)\n", video.NASPath, video.ID)
		// ... (後續的影片分析邏輯) ...
		nasBasePath, _ := filepath.Abs(s.cfg.NAS.VideoPath)
		videoAbsolutePath := filepath.Join(nasBasePath, video.NASPath)
		s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusProcessing, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{})
		tempVideoFileInfo := models.VideoFileInfo{VideoAbsolutePath: videoAbsolutePath, SourceName: video.SourceName, OriginalID: video.SourceID, VideoFileName: filepath.Base(video.NASPath)}
		tempParsedTxtData := &models.ParsedTxtData{Title: video.Title.String, ShotlistContent: video.ShotlistContent.String, Subjects: video.Subjects, Location: video.Location.String}
		promptText, promptVersion := s.buildPromptForVideo(tempVideoFileInfo, tempParsedTxtData)
		ctxVideo, cancelVideo := context.WithTimeout(context.Background(), 20*time.Minute)
		videoAnalysisResultData, geminiVideoErr := s.geminiClient.AnalyzeVideo(ctxVideo, videoAbsolutePath, promptText)
		cancelVideo()
		currentTime := time.Now()
		analyzedAtTime := sql.NullTime{Time: currentTime, Valid: true}
		if geminiVideoErr != nil {
			log.Printf("錯誤：[AnalyzeService-VideoPipeline] 使用 Gemini API 分析影片內容 %s (ID: %d, Prompt版本: %s) 失敗: %v", video.NASPath, video.ID, promptVersion, geminiVideoErr)
			errMsgSQL := sql.NullString{String: "影片內容分析失敗: " + geminiVideoErr.Error(), Valid: true}
			s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusVideoAnalysisFailed, analyzedAtTime, errMsgSQL)
			failedResult := &models.AnalysisResult{VideoID: video.ID, ErrorMessage: &models.JsonNullString{NullString: errMsgSQL}, PromptVersion: promptVersion, CreatedAt: currentTime, UpdatedAt: currentTime}
			s.db.SaveAnalysisResult(failedResult)
			failCount++
			continue
		}
		if videoAnalysisResultData == nil {
			log.Printf("警告：[AnalyzeService-VideoPipeline] GeminiClient 為影片內容 %s (ID: %d) 回傳了空的分析結果。\n", video.NASPath, video.ID)
			s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusVideoAnalysisFailed, analyzedAtTime, sql.NullString{String: "Gemini影片內容分析回傳空結果", Valid: true})
			failCount++
			continue
		}
		videoAnalysisResultData.VideoID = video.ID
		videoAnalysisResultData.PromptVersion = promptVersion
		videoAnalysisResultData.CreatedAt = currentTime
		videoAnalysisResultData.UpdatedAt = currentTime
		s.logAnalysisResult(videoAbsolutePath, videoAnalysisResultData)
		if err := s.db.SaveAnalysisResult(videoAnalysisResultData); err != nil {
			log.Printf("錯誤：[AnalyzeService-VideoPipeline] 儲存影片 ID %d 的內容分析結果到資料庫失敗: %v", video.ID, err)
			s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusVideoAnalysisFailed, analyzedAtTime, sql.NullString{String: "儲存影片內容分析結果失敗: " + err.Error(), Valid: true})
			failCount++
			continue
		}
		s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusCompleted, analyzedAtTime, sql.NullString{})
		successCount++
	}
	log.Printf("資訊：[AnalyzeService-VideoPipeline] 影片內容分析流程完成。成功: %d, 失敗: %d\n", successCount, failCount)
	return nil
}

// Run 方法 (保持不變)
func (s *AnalyzeService) Run() error { /* ... */ return nil }

// firstNChars 輔助函式
func firstNChars(s string, n int) string {
	if len(s) > n {
		runes := []rune(s)
		if len(runes) > n {
			return string(runes[:n])
		}
	}
	return s
}
