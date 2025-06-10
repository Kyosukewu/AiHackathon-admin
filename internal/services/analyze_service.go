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
	downloadPath, err := filepath.Abs(s.cfg.NAS.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("無法取得 Download 路徑的絕對路徑 '%s': %w", s.cfg.NAS.VideoPath, err)
	}
	log.Printf("資訊：[AnalyzeService] 開始掃描影片及 TXT 檔案於路徑: %s\n", downloadPath)
	supportedVideoExtensions := map[string]bool{
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".ts": true, ".flv": true, ".wmv": true,
	}
	txtExtension := ".txt"

	// 讀取 Download 目錄下的所有子資料夾
	sourceDirs, err := os.ReadDir(downloadPath)
	if err != nil {
		return nil, fmt.Errorf("[AnalyzeService] 讀取 Download 目錄 '%s' 失敗: %w", downloadPath, err)
	}

	for _, sourceDirEntry := range sourceDirs {
		if !sourceDirEntry.IsDir() {
			continue
		}
		sourceName := sourceDirEntry.Name() // 使用第一層子資料夾名稱作為 source
		sourcePath := filepath.Join(downloadPath, sourceName)

		// 掃描 source 目錄下的所有子資料夾（每個子資料夾代表一個影片ID）
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

			// 在影片ID目錄下尋找影片和 TXT 檔案
			entries, err := os.ReadDir(videoIDPath)
			if err != nil {
				log.Printf("警告：[AnalyzeService] 讀取影片ID目錄 '%s' 失敗: %v\n", videoIDPath, err)
				continue
			}

			for _, entry := range entries {
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
				relativePath, _ := filepath.Rel(downloadPath, videoFilePath)
				videoFileInfos = append(videoFileInfos, models.VideoFileInfo{
					VideoAbsolutePath: videoFilePath,
					TextFilePath:      txtFilePath,
					RelativePath:      relativePath,
					SourceName:        sourceName, // 使用第一層子資料夾名稱作為 source
					OriginalID:        videoID,    // 使用第二層子資料夾名稱作為 ID
					VideoFileName:     videoFileName,
					ModTime:           modTime,
				})
				log.Printf("資訊：[AnalyzeService] 找到匹配的影片和TXT: V: %s, T: %s (來源: %s, ID: %s)\n",
					videoFileName, filepath.Base(txtFilePath), sourceName, videoID)
			} else if txtFilePath != "" {
				// 只有 TXT 檔案的情況
				relativePath, _ := filepath.Rel(downloadPath, txtFilePath)
				videoFileInfos = append(videoFileInfos, models.VideoFileInfo{
					VideoAbsolutePath: "", // 空字串表示沒有影片檔案
					TextFilePath:      txtFilePath,
					RelativePath:      relativePath,
					SourceName:        sourceName,
					OriginalID:        videoID,
					VideoFileName:     "", // 空字串表示沒有影片檔案
					ModTime:           modTime,
				})
				log.Printf("資訊：[AnalyzeService] 找到只有 TXT 檔案的記錄: T: %s (來源: %s, ID: %s)\n",
					filepath.Base(txtFilePath), sourceName, videoID)
			} else if videoFilePath != "" {
				log.Printf("警告：[AnalyzeService] 影片ID目錄 '%s' 中只找到影片檔案。\n", videoIDPath)
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

// ExecuteTextAnalysisPipeline (修正 ok 的使用)
func (s *AnalyzeService) ExecuteTextAnalysisPipeline() error {
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

		// 先檢查資料庫中是否存在對應的 source_id 記錄
		existingVideo, getErr := s.db.GetVideoBySourceID(videoInfo.SourceName, videoInfo.OriginalID)
		if getErr != nil {
			log.Printf("錯誤：[AnalyzeService-TextPipeline] 查詢影片 SourceID %s 狀態失敗: %v. 跳過此文本分析.\n", videoInfo.OriginalID, getErr)
			failCount++
			continue
		}

		// 如果記錄存在且狀態為 completed，則跳過分析
		if existingVideo != nil && existingVideo.AnalysisStatus == models.StatusCompleted {
			log.Printf("資訊：[AnalyzeService-TextPipeline] 影片 SourceID %s 狀態為 %s，已完成分析，跳過文本分析。\n", videoInfo.OriginalID, existingVideo.AnalysisStatus)
			continue
		}

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
		existingVideo, getErr = s.db.GetVideoByID(videoID)
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
		videoToUpdate := &models.Video{
			ID:               videoID,
			SourceName:       videoInfo.SourceName,
			SourceID:         videoInfo.OriginalID,
			NASPath:          videoInfo.RelativePath,
			FetchedAt:        existingVideo.FetchedAt,
			Title:            sql.NullString{String: parsedTxtData.Title, Valid: parsedTxtData.Title != ""},
			ShotlistContent:  models.JsonNullString{NullString: sql.NullString{String: parsedTxtData.ShotlistContent, Valid: parsedTxtData.ShotlistContent != ""}},
			Location:         sql.NullString{String: parsedTxtData.Location, Valid: parsedTxtData.Location != ""},
			Restrictions:     sql.NullString{String: parsedTxtData.Restrictions, Valid: parsedTxtData.Restrictions != ""},
			TranRestrictions: sql.NullString{String: parsedTxtData.TranRestrictions, Valid: parsedTxtData.TranRestrictions != ""},
			Subjects:         parsedTxtData.Subjects,
			AnalysisStatus:   models.StatusMetadataExtracted,
			AnalyzedAt:       sql.NullTime{Time: currentTime, Valid: true},
			ViewLink:         existingVideo.ViewLink,
			SourceMetadata:   existingVideo.SourceMetadata,
			PromptVersion:    s.cfg.Prompts.TextFileAnalysis.CurrentVersion,
		}
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
	log.Println("資訊：[AnalyzeService-VideoPipeline] 開始執行影片內容分析流程...")

	// 先檢查資料庫中是否有影片
	videos, _, err := s.db.GetAllVideosWithAnalysis(100, 0, "", "", "")
	if err != nil {
		return fmt.Errorf("查詢影片失敗: %w", err)
	}
	log.Printf("資訊：[AnalyzeService-VideoPipeline] 資料庫中共有 %d 個影片", len(videos))

	// 獲取所有狀態為 metadata_extracted 的影片
	videos, _, err = s.db.GetAllVideosWithAnalysis(100, 0, "", "", string(models.StatusMetadataExtracted))
	if err != nil {
		return fmt.Errorf("查詢待分析影片失敗: %w", err)
	}
	log.Printf("資訊：[AnalyzeService-VideoPipeline] 找到 %d 個待分析的影片", len(videos))

	for _, video := range videos {
		log.Printf("資訊：[AnalyzeService-VideoPipeline] 開始處理影片 ID: %d, SourceID: %s\n", video.ID, video.SourceID)

		// 使用 nas_path 構建影片路徑
		videoPath := filepath.Join(s.cfg.NAS.VideoPath, video.NASPath)
		log.Printf("資訊：[AnalyzeService-VideoPipeline] 影片路徑: %s\n", videoPath)

		// 檢查影片檔案是否存在
		if _, err := os.Stat(videoPath); os.IsNotExist(err) {
			log.Printf("錯誤：[AnalyzeService-VideoPipeline] 影片檔案不存在: %s\n", videoPath)
			// 更新影片狀態為分析失敗
			if err := s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusVideoAnalysisFailed, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{String: fmt.Sprintf("影片檔案不存在: %s", videoPath), Valid: true}); err != nil {
				log.Printf("錯誤：[AnalyzeService-VideoPipeline] 更新影片狀態失敗: %v\n", err)
			}
			continue
		}

		// 更新影片狀態為處理中
		if err := s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusProcessing, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{}); err != nil {
			log.Printf("錯誤：[AnalyzeService-VideoPipeline] 更新影片狀態失敗: %v\n", err)
			continue
		}

		// 使用 Gemini API 分析影片
		promptText, promptVersion := s.buildPromptForVideo(models.VideoFileInfo{
			VideoAbsolutePath: videoPath,
			SourceName:        video.SourceName,
			OriginalID:        video.SourceID,
			VideoFileName:     filepath.Base(video.NASPath),
		}, &models.ParsedTxtData{
			Title:           video.Title.String,
			ShotlistContent: video.ShotlistContent.String,
			Subjects:        video.Subjects,
			Location:        video.Location.String,
		})

		analysis, err := s.geminiClient.AnalyzeVideo(context.Background(), videoPath, promptText)
		if err != nil {
			errorMsg := fmt.Sprintf("Gemini API 分析失敗: %v", err)
			log.Printf("錯誤：[AnalyzeService-VideoPipeline] %s\n", errorMsg)

			// 建立一個包含錯誤訊息的分析結果
			errorAnalysis := &models.AnalysisResult{
				VideoID:       video.ID,
				ErrorMessage:  &models.JsonNullString{NullString: sql.NullString{String: errorMsg, Valid: true}},
				PromptVersion: promptVersion,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}

			// 儲存錯誤分析結果
			if saveErr := s.db.SaveAnalysisResult(errorAnalysis); saveErr != nil {
				log.Printf("錯誤：[AnalyzeService-VideoPipeline] 儲存錯誤分析結果失敗: %v\n", saveErr)
			}

			// 更新影片狀態為分析失敗
			if err := s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusVideoAnalysisFailed, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{String: errorMsg, Valid: true}); err != nil {
				log.Printf("錯誤：[AnalyzeService-VideoPipeline] 更新影片狀態失敗: %v\n", err)
			}
			continue
		}

		// 檢查分析結果是否為空或無效
		if analysis == nil || (analysis.ShortSummary == nil && analysis.BulletedSummary == nil && analysis.VisualDescription == nil) {
			errorMsg := "Gemini API 回傳的分析結果為空或無效"
			log.Printf("錯誤：[AnalyzeService-VideoPipeline] %s\n", errorMsg)

			// 建立一個包含錯誤訊息的分析結果
			errorAnalysis := &models.AnalysisResult{
				VideoID:       video.ID,
				ErrorMessage:  &models.JsonNullString{NullString: sql.NullString{String: errorMsg, Valid: true}},
				PromptVersion: promptVersion,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}

			// 儲存錯誤分析結果
			if saveErr := s.db.SaveAnalysisResult(errorAnalysis); saveErr != nil {
				log.Printf("錯誤：[AnalyzeService-VideoPipeline] 儲存錯誤分析結果失敗: %v\n", saveErr)
			}

			// 更新影片狀態為分析失敗
			if err := s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusVideoAnalysisFailed, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{String: errorMsg, Valid: true}); err != nil {
				log.Printf("錯誤：[AnalyzeService-VideoPipeline] 更新影片狀態失敗: %v\n", err)
			}
			continue
		}

		// 設置分析結果的額外資訊
		analysis.VideoID = video.ID
		analysis.PromptVersion = promptVersion
		analysis.CreatedAt = time.Now()
		analysis.UpdatedAt = time.Now()

		// 保存分析結果
		if err := s.db.SaveAnalysisResult(analysis); err != nil {
			errorMsg := fmt.Sprintf("保存分析結果失敗: %v", err)
			log.Printf("錯誤：[AnalyzeService-VideoPipeline] %s\n", errorMsg)

			// 更新影片狀態為分析失敗
			if err := s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusVideoAnalysisFailed, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{String: errorMsg, Valid: true}); err != nil {
				log.Printf("錯誤：[AnalyzeService-VideoPipeline] 更新影片狀態失敗: %v\n", err)
			}
			continue
		}

		// 更新影片狀態為分析完成
		if err := s.db.UpdateVideoAnalysisStatus(video.ID, models.StatusCompleted, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{}); err != nil {
			log.Printf("錯誤：[AnalyzeService-VideoPipeline] 更新影片狀態失敗: %v\n", err)
			continue
		}

		log.Printf("資訊：[AnalyzeService-VideoPipeline] 影片 ID: %d 分析完成\n", video.ID)
	}

	log.Println("資訊：[AnalyzeService-VideoPipeline] 影片內容分析流程執行完成")
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
