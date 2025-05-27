package mysql

import (
	"AiHackathon-admin/internal/config"
	"AiHackathon-admin/internal/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLStore 結構 (保持不變)
type MySQLStore struct{ db *sql.DB }

// NewMySQLStore, Close (保持不變)
func NewMySQLStore(dbCfg config.DatabaseConfig) (*MySQLStore, error) {
	if dbCfg.Driver != "mysql" {
		return nil, fmt.Errorf("不支援的資料庫驅動程式: %s", dbCfg.Driver)
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("開啟資料庫連線失敗: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("無法連線到資料庫: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	log.Println("資訊：成功連線到 MySQL 資料庫。")
	return &MySQLStore{db: db}, nil
}
func (s *MySQLStore) Close() error {
	if s.db != nil {
		log.Println("資訊：正在關閉 MySQL 資料庫連線...")
		return s.db.Close()
	}
	return nil
}

func (s *MySQLStore) GetAllVideosWithAnalysis(limit int, offset int) ([]models.Video, []models.AnalysisResult, error) {
	log.Printf("資訊：MySQLStore.GetAllVideosWithAnalysis 被呼叫 (limit: %d, offset: %d)\n", limit, offset)
	query := `
		SELECT
			v.id, v.source_name, v.source_id, v.nas_path, v.title, v.fetched_at, v.analysis_status, v.analyzed_at, v.source_metadata,
			ar.video_id, ar.transcript, ar.translation, ar.summary, ar.visual_description, ar.topics, ar.keywords, ar.error_message,
			ar.prompt_version, ar.created_at, ar.updated_at
		FROM videos v
		LEFT JOIN analysis_results ar ON v.id = ar.video_id
		ORDER BY v.fetched_at DESC
		LIMIT ? OFFSET ?;`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, nil, fmt.Errorf("查詢影片和分析結果失敗: %w", err)
	}
	defer rows.Close()

	var videos []models.Video
	analysisResultMap := make(map[int64]models.AnalysisResult)

	for rows.Next() {
		var v models.Video
		var arTemp models.AnalysisResult

		var sourceMetadataSQL sql.RawBytes // 使用 sql.RawBytes 處理 nullable JSON
		var arVideoID sql.NullInt64
		var arTranscriptSQL, arTranslationSQL, arSummarySQL, arVisualDescriptionSQL, arErrorMessageSQL sql.NullString
		var arTopicsSQL, arKeywordsSQL sql.RawBytes // 使用 sql.RawBytes 處理 nullable JSON
		var arPromptVersionSQL sql.NullString
		var arCreatedAt, arUpdatedAt sql.NullTime

		err := rows.Scan(
			&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title, &v.FetchedAt, &v.AnalysisStatus, &v.AnalyzedAt,
			&sourceMetadataSQL, // 掃描到 sql.RawBytes
			&arVideoID,
			&arTranscriptSQL, &arTranslationSQL, &arSummarySQL, &arVisualDescriptionSQL,
			&arTopicsSQL,   // 掃描到 sql.RawBytes
			&arKeywordsSQL, // 掃描到 sql.RawBytes
			&arErrorMessageSQL,
			&arPromptVersionSQL,
			&arCreatedAt, &arUpdatedAt,
		)
		if err != nil {
			log.Printf("錯誤：掃描查詢結果行失敗: %v", err)
			continue
		}

		if sourceMetadataSQL != nil { // 如果不是 DB NULL
			v.SourceMetadata = json.RawMessage(sourceMetadataSQL)
		}

		videos = appendIfMissingVideo(videos, v)

		if arVideoID.Valid {
			arTemp.VideoID = arVideoID.Int64
			// 如果 sql.NullString 有效，則創建 *JsonNullString 實例
			if arTranscriptSQL.Valid {
				arTemp.Transcript = &models.JsonNullString{NullString: arTranscriptSQL}
			}
			if arTranslationSQL.Valid {
				arTemp.Translation = &models.JsonNullString{NullString: arTranslationSQL}
			}
			if arSummarySQL.Valid {
				arTemp.Summary = &models.JsonNullString{NullString: arSummarySQL}
			}
			if arVisualDescriptionSQL.Valid {
				arTemp.VisualDescription = &models.JsonNullString{NullString: arVisualDescriptionSQL}
			}
			if arErrorMessageSQL.Valid {
				arTemp.ErrorMessage = &models.JsonNullString{NullString: arErrorMessageSQL}
			}

			if arTopicsSQL != nil {
				arTemp.Topics = json.RawMessage(arTopicsSQL)
			}
			if arKeywordsSQL != nil {
				arTemp.Keywords = json.RawMessage(arKeywordsSQL)
			}

			if arPromptVersionSQL.Valid {
				arTemp.PromptVersion = arPromptVersionSQL.String
			} else {
				arTemp.PromptVersion = ""
			}

			if arCreatedAt.Valid {
				arTemp.CreatedAt = arCreatedAt.Time
			}
			if arUpdatedAt.Valid {
				arTemp.UpdatedAt = arUpdatedAt.Time
			}
			analysisResultMap[arTemp.VideoID] = arTemp
		}
	}
	if err = rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("處理查詢結果集時發生錯誤: %w", err)
	}
	var finalAnalysisResults []models.AnalysisResult
	for _, v_ := range videos {
		if ar, ok := analysisResultMap[v_.ID]; ok {
			finalAnalysisResults = append(finalAnalysisResults, ar)
		}
	}
	log.Printf("資訊：查詢到 %d 個影片，%d 個有效分析結果。\n", len(videos), len(finalAnalysisResults))
	return videos, finalAnalysisResults, nil
}

func appendIfMissingVideo(slice []models.Video, v models.Video) []models.Video {
	for _, existing := range slice {
		if existing.ID == v.ID {
			return slice
		}
	}
	return append(slice, v)
}
func (s *MySQLStore) FindOrCreateVideo(videoInfo models.VideoFileInfo) (int64, error) { /* ... 保持不變 ... */
	var videoID int64
	query := "SELECT id FROM videos WHERE nas_path = ?"
	err := s.db.QueryRow(query, videoInfo.RelativePath).Scan(&videoID)
	if err == sql.ErrNoRows {
		insertQuery := `INSERT INTO videos (source_name, source_id, nas_path, title, fetched_at, analysis_status, source_metadata) VALUES (?, ?, ?, ?, ?, ?, ?);`
		var title sql.NullString
		if videoInfo.FileName != "" {
			title = sql.NullString{String: videoInfo.FileName, Valid: true}
		}
		fetchedTime := videoInfo.ModTime
		if fetchedTime.IsZero() {
			fetchedTime = time.Now()
		}
		res, insertErr := s.db.Exec(insertQuery, videoInfo.SourceName, videoInfo.OriginalID, videoInfo.RelativePath, title, fetchedTime, models.StatusPending, nil)
		if insertErr != nil {
			return 0, fmt.Errorf("插入新影片記錄失敗: %w", insertErr)
		}
		videoID, insertErr = res.LastInsertId()
		if insertErr != nil {
			return 0, fmt.Errorf("獲取新插入影片的 ID 失敗: %w", insertErr)
		}
		log.Printf("資訊：新增影片記錄成功，ID: %d (NAS 路徑: %s)\n", videoID, videoInfo.RelativePath)
		return videoID, nil
	} else if err != nil {
		return 0, fmt.Errorf("查找影片失敗: %w", err)
	}
	log.Printf("資訊：資料庫中已存在影片記錄，ID: %d (NAS 路徑: %s)\n", videoID, videoInfo.RelativePath)
	return videoID, nil
}

// SaveAnalysisResult 儲存分析結果 (處理 *JsonNullString)
func (s *MySQLStore) SaveAnalysisResult(result *models.AnalysisResult) error {
	if result == nil || result.VideoID == 0 {
		return fmt.Errorf("無效的分析結果或 VideoID 為空")
	}
	query := `
		INSERT INTO analysis_results (
			video_id, transcript, translation, summary, visual_description, 
			topics, keywords, error_message, prompt_version, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			transcript = VALUES(transcript), translation = VALUES(translation), summary = VALUES(summary),
			visual_description = VALUES(visual_description), topics = VALUES(topics), keywords = VALUES(keywords),
			error_message = VALUES(error_message), prompt_version = VALUES(prompt_version),
			updated_at = VALUES(updated_at);`

	// 處理指標類型，如果為 nil，則傳遞 sql.NullString{Valid: false} 給資料庫
	var transcript, translation, summary, visualDesc, errMsg sql.NullString
	if result.Transcript != nil {
		transcript = result.Transcript.NullString
	}
	if result.Translation != nil {
		translation = result.Translation.NullString
	}
	if result.Summary != nil {
		summary = result.Summary.NullString
	}
	if result.VisualDescription != nil {
		visualDesc = result.VisualDescription.NullString
	}
	if result.ErrorMessage != nil {
		errMsg = result.ErrorMessage.NullString
	}

	var promptVersion sql.NullString
	if result.PromptVersion != "" {
		promptVersion = sql.NullString{String: result.PromptVersion, Valid: true}
	}

	_, err := s.db.Exec(query,
		result.VideoID,
		transcript,      // 傳遞 sql.NullString
		translation,     // 傳遞 sql.NullString
		summary,         // 傳遞 sql.NullString
		visualDesc,      // 傳遞 sql.NullString
		result.Topics,   // json.RawMessage (可以是 nil)
		result.Keywords, // json.RawMessage (可以是 nil)
		errMsg,          // 傳遞 sql.NullString
		promptVersion,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("儲存分析結果到資料庫失敗 (VideoID: %d): %w", result.VideoID, err)
	}
	log.Printf("資訊：分析結果成功儲存到資料庫 (VideoID: %d, PromptVersion: %s)\n", result.VideoID, result.PromptVersion)
	return nil
}
func (s *MySQLStore) UpdateVideoAnalysisStatus(videoID int64, status models.AnalysisStatus, analyzedAt sql.NullTime, errorMessage sql.NullString) error { /* ... 保持不變 ... */
	if videoID == 0 {
		return fmt.Errorf("無效的 VideoID")
	}
	query := "UPDATE videos SET analysis_status = ?, analyzed_at = ? WHERE id = ?"
	params := []interface{}{status, analyzedAt, videoID}
	_, err := s.db.Exec(query, params...)
	if err != nil {
		return fmt.Errorf("更新影片分析狀態失敗 (VideoID: %d, Status: %s): %w", videoID, status, err)
	}
	log.Printf("資訊：影片分析狀態成功更新 (VideoID: %d, Status: %s)\n", videoID, status)
	return nil
}
func (s *MySQLStore) GetPendingVideos(limit int) ([]models.Video, error) { /* ... 保持不變 ... */
	query := `SELECT id, source_name, source_id, nas_path, title, fetched_at, analysis_status, analyzed_at, source_metadata FROM videos WHERE analysis_status = ? ORDER BY fetched_at ASC LIMIT ?;`
	rows, err := s.db.Query(query, models.StatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("查詢待處理影片失敗: %w", err)
	}
	defer rows.Close()
	var videos []models.Video
	for rows.Next() {
		var v models.Video
		var sourceMetadataBytes []byte
		err := rows.Scan(&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title, &v.FetchedAt, &v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataBytes)
		if err != nil {
			log.Printf("錯誤：掃描待處理影片查詢結果行失敗: %v", err)
			continue
		}
		if sourceMetadataBytes != nil {
			v.SourceMetadata = json.RawMessage(sourceMetadataBytes)
		}
		videos = append(videos, v)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("處理待處理影片查詢結果集時發生錯誤: %w", err)
	}
	log.Printf("資訊：查詢到 %d 個待處理影片。\n", len(videos))
	return videos, nil
}
func (s *MySQLStore) GetVideoByID(videoID int64) (*models.Video, error) { /* ... 保持不變 ... */
	if videoID == 0 {
		return nil, fmt.Errorf("無效的 VideoID")
	}
	query := `SELECT id, source_name, source_id, nas_path, title, fetched_at, analysis_status, analyzed_at, source_metadata FROM videos WHERE id = ?;`
	row := s.db.QueryRow(query, videoID)
	var v models.Video
	var sourceMetadataBytes []byte
	err := row.Scan(&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title, &v.FetchedAt, &v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查詢 VideoID %d 失敗: %w", videoID, err)
	}
	if sourceMetadataBytes != nil {
		v.SourceMetadata = json.RawMessage(sourceMetadataBytes)
	}
	return &v, nil
}
