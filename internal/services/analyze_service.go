package services

import (
	"AiHackathon-admin/internal/clients/gemini"
	"AiHackathon-admin/internal/config"
	"AiHackathon-admin/internal/models"
	"AiHackathon-admin/internal/web/handlers"
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AnalyzeService 結構 (保持不變)
type AnalyzeService struct {
	cfg          *config.Config
	db           handlers.DBStore
	nas          NASStorage
	geminiClient *gemini.Client
}

// NewAnalyzeService (保持不變)
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
	return &AnalyzeService{cfg: cfg, db: db, nas: nas, geminiClient: geminiClient}, nil
}

// scanVideoFiles (保持不變)
func (s *AnalyzeService) scanVideoFiles() ([]models.VideoFileInfo, error) {
	var videoFiles []models.VideoFileInfo
	nasBasePath, err := filepath.Abs(s.cfg.NAS.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("無法取得 NAS videoPath 的絕對路徑 '%s': %w", s.cfg.NAS.VideoPath, err)
	}
	log.Printf("資訊：[AnalyzeService] 開始掃描影片檔案於路徑: %s (簡化模式)\n", nasBasePath)
	supportedExtensions := map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".ts": true, ".flv": true, ".wmv": true}
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
		err = filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				log.Printf("警告：[AnalyzeService] 訪問路徑 %s 時發生錯誤: %v (已跳過)\n", path, walkErr)
				if os.IsPermission(walkErr) {
					return filepath.SkipDir
				}
				return nil
			}
			if path == sourcePath && d.IsDir() {
				return nil
			}
			if d.IsDir() {
				return filepath.SkipDir
			}
			ext := strings.ToLower(filepath.Ext(path))
			if supportedExtensions[ext] {
				relativePath, relErr := filepath.Rel(nasBasePath, path)
				if relErr != nil {
					log.Printf("警告：[AnalyzeService] 無法取得檔案 '%s' 相對於 '%s' 的路徑: %v (已跳過)\n", path, nasBasePath, relErr)
					return nil
				}
				fileName := d.Name()
				originalID := strings.TrimSuffix(fileName, ext)
				fileInfo, statErr := d.Info()
				var modTime time.Time
				if statErr == nil {
					modTime = fileInfo.ModTime()
				} else {
					osStatInfo, osStatErr := os.Stat(path)
					if osStatErr == nil {
						modTime = osStatInfo.ModTime()
					} else {
						log.Printf("警告：[AnalyzeService] 無法獲取檔案 '%s' 的詳細資訊 (d.Info: %v, os.Stat: %v)\n", path, statErr, osStatErr)
					}
				}
				videoFiles = append(videoFiles, models.VideoFileInfo{
					AbsolutePath: path, RelativePath: relativePath, SourceName: sourceName,
					OriginalID: originalID, FileName: fileName, ModTime: modTime,
				})
				log.Printf("資訊：[AnalyzeService] 找到影片檔案: %s (來源: %s, ID: %s)\n", path, sourceName, originalID)
			}
			return nil
		})
		if err != nil {
			log.Printf("警告：[AnalyzeService] 遍歷來源目錄 '%s' 時發生錯誤: %v\n", sourcePath, err)
		}
	}
	log.Printf("資訊：[AnalyzeService] 掃描完成，共找到 %d 個影片檔案。\n", len(videoFiles))
	return videoFiles, nil
}

// buildPromptForVideo (保持不變)
func (s *AnalyzeService) buildPromptForVideo(videoInfo models.VideoFileInfo) (promptText string, promptVersion string) {
	currentVersionKey := s.cfg.Prompts.VideoAnalysis.CurrentVersion
	if currentVersionKey == "" {
		log.Println("警告：[AnalyzeService] 設定檔中未指定 currentVersion for videoAnalysis Prompt，將使用預設 Prompt。")
		return "請分析此影片。", "default-fallback-v0"
	}
	promptText, ok := s.cfg.Prompts.VideoAnalysis.Versions[currentVersionKey]
	if !ok || promptText == "" {
		log.Printf("警告：[AnalyzeService] 設定檔中未找到名為 '%s' 的 videoAnalysis Prompt 版本，或其內容為空。將使用預設 Prompt。", currentVersionKey)
		return "請分析此影片。", "default-fallback-v0"
	}
	log.Printf("資訊：[AnalyzeService] 使用 Prompt 版本: %s\n", currentVersionKey)
	return promptText, currentVersionKey
}

// logAnalysisResult (保持不變)
func (s *AnalyzeService) logAnalysisResult(videoPath string, result *models.AnalysisResult) {
	if result == nil {
		log.Printf("資訊：[AnalyzeService] 影片 %s 沒有分析結果可供記錄。\n", videoPath)
		return
	}
	log.Printf("--- [AnalyzeService] 影片分析結果 (%s) ---", videoPath)
	if result.PromptVersion != "" {
		log.Printf("Prompt 版本: %s\n", result.PromptVersion)
	}
	if result.Transcript != nil && result.Transcript.Valid {
		log.Printf("逐字稿: %s\n", result.Transcript.String)
	}
	if result.Translation != nil && result.Translation.Valid {
		log.Printf("翻譯: %s\n", result.Translation.String)
	}
	if result.Summary != nil && result.Summary.Valid {
		log.Printf("摘要: %s\n", result.Summary.String)
	}
	if result.VisualDescription != nil && result.VisualDescription.Valid {
		log.Printf("畫面描述: %s\n", result.VisualDescription.String)
	}
	if len(result.Topics) > 0 {
		log.Printf("主題: %s\n", string(result.Topics))
	}
	if len(result.Keywords) > 0 {
		log.Printf("關鍵詞: %s\n", string(result.Keywords))
	}
	if result.ErrorMessage != nil && result.ErrorMessage.Valid {
		log.Printf("錯誤訊息: %s\n", result.ErrorMessage.String)
	}
	log.Println("--- [AnalyzeService] 分析結果結束 ---")
}

// Run 方法 (修正 ErrorMessage 賦值)
func (s *AnalyzeService) Run() error {
	log.Println("資訊：[AnalyzeService] 影片分析服務執行中...")

	videoFileInfos, err := s.scanVideoFiles()
	if err != nil {
		log.Printf("錯誤：[AnalyzeService] 掃描影片檔案失敗: %v", err)
		return err
	}

	if len(videoFileInfos) == 0 {
		log.Println("資訊：[AnalyzeService] 在 NAS 中沒有找到影片檔案進行分析。")
		return nil
	}

	log.Printf("資訊：[AnalyzeService] 找到 %d 個影片檔案準備分析。\n", len(videoFileInfos))

	for _, videoInfo := range videoFileInfos {
		log.Printf("資訊：[AnalyzeService] 開始處理影片: %s (來源: %s, ID: %s)\n", videoInfo.AbsolutePath, videoInfo.SourceName, videoInfo.OriginalID)

		videoID, err := s.db.FindOrCreateVideo(videoInfo)
		if err != nil {
			log.Printf("錯誤：[AnalyzeService] 查找或建立影片 '%s' 的資料庫記錄失敗: %v", videoInfo.RelativePath, err)
			continue
		}
		log.Printf("資訊：[AnalyzeService] 影片 '%s' 對應的資料庫 ID: %d\n", videoInfo.RelativePath, videoID)

		existingVideo, err := s.db.GetVideoByID(videoID)
		if err != nil {
			log.Printf("錯誤：[AnalyzeService] 查詢影片 ID %d 的狀態失敗: %v。將繼續嘗試分析...", videoID, err)
		} else if existingVideo != nil && existingVideo.AnalysisStatus == models.StatusCompleted {
			log.Printf("資訊：[AnalyzeService] 影片 ID %d (路徑 %s) 狀態為 '%s'，跳過分析。\n", videoID, videoInfo.RelativePath, existingVideo.AnalysisStatus)
			continue
		}

		if existingVideo == nil || (existingVideo.AnalysisStatus != models.StatusCompleted && existingVideo.AnalysisStatus != models.StatusFailed) {
			err = s.db.UpdateVideoAnalysisStatus(videoID, models.StatusProcessing, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{})
			if err != nil {
				log.Printf("錯誤：[AnalyzeService] 更新影片 ID %d 狀態為 'processing' 失敗: %v", videoID, err)
			}
		} else if existingVideo != nil && existingVideo.AnalysisStatus == models.StatusFailed {
			log.Printf("資訊：[AnalyzeService] 影片 ID %d (路徑 %s) 之前分析失敗，將嘗試重新分析。\n", videoID, videoInfo.RelativePath)
			err = s.db.UpdateVideoAnalysisStatus(videoID, models.StatusProcessing, sql.NullTime{Time: time.Now(), Valid: true}, sql.NullString{})
			if err != nil {
				log.Printf("錯誤：[AnalyzeService] 更新失敗影片 ID %d 狀態為 'processing' 失敗: %v", videoID, err)
			}
		}

		promptText, promptVersion := s.buildPromptForVideo(videoInfo)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		analysisResultData, geminiErr := s.geminiClient.AnalyzeVideo(ctx, videoInfo.AbsolutePath, promptText)
		cancel()

		if geminiErr != nil {
			log.Printf("錯誤：[AnalyzeService] 使用 Gemini API 分析影片 %s (Prompt版本: %s) 失敗: %v", videoInfo.AbsolutePath, promptVersion, geminiErr)
			errMsgSQL := sql.NullString{String: geminiErr.Error(), Valid: true}
			analyzedTime := sql.NullTime{Time: time.Now(), Valid: true}
			// 注意：UpdateVideoAnalysisStatus 的最後一個參數是 errorMessage，類型是 sql.NullString
			if dbErr := s.db.UpdateVideoAnalysisStatus(videoID, models.StatusFailed, analyzedTime, errMsgSQL); dbErr != nil {
				log.Printf("錯誤：[AnalyzeService] 更新影片 ID %d 狀態為 'failed' 失敗: %v", videoID, dbErr)
			}
			// --- 修正 ErrorMessage 賦值 ---
			failedResult := &models.AnalysisResult{
				VideoID:       videoID,
				ErrorMessage:  &models.JsonNullString{NullString: errMsgSQL}, // <--- 修改此處，取地址
				PromptVersion: promptVersion,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}
			// --- 結束修正 ---
			if dbErr := s.db.SaveAnalysisResult(failedResult); dbErr != nil {
				log.Printf("錯誤：[AnalyzeService] 儲存影片 ID %d 的失敗分析結果到資料庫失敗: %v", videoID, dbErr)
			}
			continue
		}

		log.Printf("資訊：[AnalyzeService] 影片 %s (Prompt版本: %s) 分析成功。\n", videoInfo.AbsolutePath, promptVersion)

		// analysisResultData 是 *models.AnalysisResult，其 JsonNullString 欄位已經是指標了
		// 如果 geminiClient.AnalyzeVideo 回傳的 analysisResultData 中的 JsonNullString 欄位是 nil，
		// 則它們在 SaveAnalysisResult 時會被正確處理為 NULL。
		analysisResultData.VideoID = videoID
		analysisResultData.PromptVersion = promptVersion
		analysisResultData.CreatedAt = time.Now()
		analysisResultData.UpdatedAt = time.Now()

		s.logAnalysisResult(videoInfo.AbsolutePath, analysisResultData)

		if err := s.db.SaveAnalysisResult(analysisResultData); err != nil {
			log.Printf("錯誤：[AnalyzeService] 儲存影片 ID %d 的分析結果到資料庫失敗: %v", videoID, err)
			continue
		}

		analyzedTime := sql.NullTime{Time: time.Now(), Valid: true}
		if err := s.db.UpdateVideoAnalysisStatus(videoID, models.StatusCompleted, analyzedTime, sql.NullString{}); err != nil {
			log.Printf("錯誤：[AnalyzeService] 更新影片 ID %d 狀態為 'completed' 失敗: %v", videoID, err)
		}
	}

	log.Println("資訊：[AnalyzeService] 影片分析服務本輪執行完成。")
	return nil
}
