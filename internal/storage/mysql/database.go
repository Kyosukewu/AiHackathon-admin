package mysql

import (
	"AiHackathon-admin/internal/config"
	"AiHackathon-admin/internal/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql" // 匿名引入 MySQL 驅動程式
)

// MySQLStore 結構負責與 MySQL 資料庫互動
type MySQLStore struct {
	db *sql.DB // 資料庫連線池
}

// NewMySQLStore 建立一個 MySQLStore 實例並初始化資料庫連線
func NewMySQLStore(dbCfg config.DatabaseConfig) (*MySQLStore, error) {
	if dbCfg.Driver != "mysql" {
		return nil, fmt.Errorf("不支援的資料庫驅動程式: %s，目前僅支援 mysql", dbCfg.Driver)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbCfg.User,
		dbCfg.Password,
		dbCfg.Host,
		dbCfg.Port,
		dbCfg.DBName,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("開啟資料庫連線失敗: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close() // 如果 ping 失敗，關閉已開啟的連線
		return nil, fmt.Errorf("無法連線到資料庫 (ping 失敗): %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("資訊：成功連線到 MySQL 資料庫。")
	return &MySQLStore{db: db}, nil
}

// Close 關閉資料庫連線
func (s *MySQLStore) Close() error {
	if s.db != nil {
		log.Println("資訊：正在關閉 MySQL 資料庫連線...")
		return s.db.Close()
	}
	return nil
}

// GetAllVideosWithAnalysis 從資料庫獲取影片及其分析結果
func (s *MySQLStore) GetAllVideosWithAnalysis(limit int, offset int) ([]models.Video, []models.AnalysisResult, error) {
	log.Printf("資訊：MySQLStore.GetAllVideosWithAnalysis 被呼叫 (limit: %d, offset: %d)\n", limit, offset)

	query := `
		SELECT
			v.id, v.source_name, v.source_id, v.nas_path, v.title, 
			v.fetched_at, v.published_at, v.duration_secs, v.shotlist_content, v.view_link,
			v.analysis_status, v.analyzed_at, v.source_metadata,
			v.subjects, v.location, 
			ar.video_id, ar.transcript, ar.translation, 
			ar.short_summary, ar.bulleted_summary, ar.bites, ar.mentioned_locations,
			ar.importance_score, ar.material_type, ar.related_news,
			ar.visual_description, ar.topics, ar.keywords, ar.error_message,
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

		var sourceMetadataSQL, subjectsSQL sql.RawBytes
		var shotlistContentSQL, viewLinkSQL, locationSQL sql.NullString

		var arVideoID sql.NullInt64
		var arTranscriptSQL, arTranslationSQL, arShortSummarySQL, arBulletedSummarySQL, arMaterialTypeSQL, arVisualDescriptionSQL, arErrorMessageSQL, arPromptVersionSQL sql.NullString
		var arTopicsSQL, arKeywordsSQL, arBitesSQL, arMentionedLocationsSQL, arImportanceScoreSQL, arRelatedNewsSQL sql.RawBytes
		var arCreatedAt, arUpdatedAt sql.NullTime

		err := rows.Scan(
			&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title,
			&v.FetchedAt, &v.PublishedAt, &v.DurationSecs, &shotlistContentSQL, &viewLinkSQL,
			&v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataSQL,
			&subjectsSQL, &locationSQL,
			&arVideoID,
			&arTranscriptSQL, &arTranslationSQL,
			&arShortSummarySQL, &arBulletedSummarySQL, &arBitesSQL, &arMentionedLocationsSQL,
			&arImportanceScoreSQL, &arMaterialTypeSQL, &arRelatedNewsSQL,
			&arVisualDescriptionSQL,
			&arTopicsSQL, &arKeywordsSQL, &arErrorMessageSQL,
			&arPromptVersionSQL, &arCreatedAt, &arUpdatedAt,
		)
		if err != nil {
			log.Printf("錯誤：掃描查詢結果行失敗: %v", err)
			continue
		}

		if sourceMetadataSQL != nil {
			v.SourceMetadata = json.RawMessage(sourceMetadataSQL)
		}
		if subjectsSQL != nil {
			v.Subjects = json.RawMessage(subjectsSQL)
		}
		v.ShotlistContent = models.JsonNullString{NullString: shotlistContentSQL}
		v.Location = locationSQL
		if viewLinkSQL.Valid {
			v.ViewLink = viewLinkSQL
		}

		var videoExists bool
		for _, existingVideo := range videos {
			if existingVideo.ID == v.ID {
				videoExists = true
				break
			}
		}
		if !videoExists {
			videos = append(videos, v)
		}

		if arVideoID.Valid {
			arTemp.VideoID = arVideoID.Int64
			if arTranscriptSQL.Valid {
				arTemp.Transcript = &models.JsonNullString{NullString: arTranscriptSQL}
			} else {
				arTemp.Transcript = nil
			}
			if arTranslationSQL.Valid {
				arTemp.Translation = &models.JsonNullString{NullString: arTranslationSQL}
			} else {
				arTemp.Translation = nil
			}
			if arShortSummarySQL.Valid {
				arTemp.ShortSummary = &models.JsonNullString{NullString: arShortSummarySQL}
			} else {
				arTemp.ShortSummary = nil
			}
			if arBulletedSummarySQL.Valid {
				arTemp.BulletedSummary = &models.JsonNullString{NullString: arBulletedSummarySQL}
			} else {
				arTemp.BulletedSummary = nil
			}
			if arMaterialTypeSQL.Valid {
				arTemp.MaterialType = &models.JsonNullString{NullString: arMaterialTypeSQL}
			} else {
				arTemp.MaterialType = nil
			}
			if arVisualDescriptionSQL.Valid {
				arTemp.VisualDescription = &models.JsonNullString{NullString: arVisualDescriptionSQL}
			} else {
				arTemp.VisualDescription = nil
			}
			if arErrorMessageSQL.Valid {
				arTemp.ErrorMessage = &models.JsonNullString{NullString: arErrorMessageSQL}
			} else {
				arTemp.ErrorMessage = nil
			}

			if arBitesSQL != nil {
				arTemp.Bites = json.RawMessage(arBitesSQL)
			} else {
				arTemp.Bites = nil
			}
			if arMentionedLocationsSQL != nil {
				arTemp.MentionedLocations = json.RawMessage(arMentionedLocationsSQL)
			} else {
				arTemp.MentionedLocations = nil
			}
			if arImportanceScoreSQL != nil {
				arTemp.ImportanceScore = json.RawMessage(arImportanceScoreSQL)
			} else {
				arTemp.ImportanceScore = nil
			}
			if arRelatedNewsSQL != nil {
				arTemp.RelatedNews = json.RawMessage(arRelatedNewsSQL)
			} else {
				arTemp.RelatedNews = nil
			}
			if arTopicsSQL != nil {
				arTemp.Topics = json.RawMessage(arTopicsSQL)
			} else {
				arTemp.Topics = nil
			}
			if arKeywordsSQL != nil {
				arTemp.Keywords = json.RawMessage(arKeywordsSQL)
			} else {
				arTemp.Keywords = nil
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

// FindOrCreateVideo 更新：接收 *models.Video 並在 INSERT/UPDATE 時使用其所有欄位
func (s *MySQLStore) FindOrCreateVideo(video *models.Video) (int64, error) {
	if video == nil {
		return 0, fmt.Errorf("傳入的 video 物件不得為 nil")
	}
	if video.NASPath == "" && (video.SourceName == "" || video.SourceID == "") {
		return 0, fmt.Errorf("video 物件的 NASPath 或 SourceName+SourceID 必須提供至少一組")
	}

	var videoID int64
	var queryErr error

	if video.SourceName != "" && video.SourceID != "" {
		query := "SELECT id FROM videos WHERE source_name = ? AND source_id = ?"
		queryErr = s.db.QueryRow(query, video.SourceName, video.SourceID).Scan(&videoID)
	} else if video.NASPath != "" { // 如果沒有 SourceName/ID，則使用 NASPath
		query := "SELECT id FROM videos WHERE nas_path = ?"
		queryErr = s.db.QueryRow(query, video.NASPath).Scan(&videoID)
	} else {
		// 這種情況不應該發生，因為我們在函數開頭檢查了
		return 0, fmt.Errorf("無法確定查找影片的唯一標識 (NASPath 或 SourceName+SourceID 均為空)")
	}

	if queryErr == sql.ErrNoRows { // 影片不存在，插入新記錄
		log.Printf("資訊：資料庫中未找到影片 (Source: %s, ID: %s, NAS: %s)，正在新增記錄...\n", video.SourceName, video.SourceID, video.NASPath)
		insertQuery := `
			INSERT INTO videos (
				source_name, source_id, nas_path, title, fetched_at, 
				published_at, duration_secs, shotlist_content, view_link, 
				subjects, location, 
				analysis_status, source_metadata, analyzed_at 
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

		fetchedTime := video.FetchedAt
		if fetchedTime.IsZero() {
			fetchedTime = time.Now()
		}
		status := video.AnalysisStatus
		if status == "" {
			status = models.StatusPending
		}

		res, insertErr := s.db.Exec(insertQuery,
			video.SourceName, video.SourceID, video.NASPath,
			video.Title, fetchedTime, video.PublishedAt, video.DurationSecs,
			video.ShotlistContent, video.ViewLink, video.Subjects, video.Location,
			status, video.SourceMetadata, video.AnalyzedAt,
		)
		if insertErr != nil {
			return 0, fmt.Errorf("插入新影片記錄失敗 (Source: %s, ID: %s): %w", video.SourceName, video.SourceID, insertErr)
		}
		videoID, insertErr = res.LastInsertId()
		if insertErr != nil {
			return 0, fmt.Errorf("獲取新插入影片的 ID 失敗 (Source: %s, ID: %s): %w", video.SourceName, video.SourceID, insertErr)
		}
		log.Printf("資訊：新增影片記錄成功，ID: %d (Source: %s, ID: %s)\n", videoID, video.SourceName, video.SourceID)
		return videoID, nil // 確保這裡有 return

	} else if queryErr != nil { // 查詢時發生其他錯誤
		return 0, fmt.Errorf("查找影片失敗 (Source: %s, ID: %s): %w", video.SourceName, video.SourceID, queryErr)
	}

	// 影片已存在，執行更新元數據操作
	log.Printf("資訊：資料庫中已存在影片記錄 ID: %d (Source: %s, ID: %s)。正在更新元數據...\n", videoID, video.SourceName, video.SourceID)

	// 只更新那些通常來自 TXT 或外部元數據的欄位
	// fetched_at, analysis_status, analyzed_at 通常由內部流程控制，
	// 但如果傳入的 video 物件中包含這些值，也應該更新它們以反映最新狀態。
	updateQuery := `
		UPDATE videos SET
			title = ?, published_at = ?, duration_secs = ?, shotlist_content = ?, 
			view_link = ?, subjects = ?, location = ?, 
			nas_path = ?, source_metadata = ?, fetched_at = ?, 
			analysis_status = ?, analyzed_at = ? 
		WHERE id = ?;`

	_, updateErr := s.db.Exec(updateQuery,
		video.Title, video.PublishedAt, video.DurationSecs, video.ShotlistContent,
		video.ViewLink, video.Subjects, video.Location,
		video.NASPath, video.SourceMetadata, video.FetchedAt,
		video.AnalysisStatus, video.AnalyzedAt,
		videoID,
	)
	if updateErr != nil {
		return 0, fmt.Errorf("更新影片 ID %d 的元數據失敗: %w", videoID, updateErr)
	}
	log.Printf("資訊：影片 ID %d 的元數據更新成功。\n", videoID)
	return videoID, nil // 確保這裡有 return
}

// SaveAnalysisResult (輔助函式 toSQLNullString 已包含在此處，因為它只在這裡使用)
func (s *MySQLStore) SaveAnalysisResult(result *models.AnalysisResult) error {
	if result == nil || result.VideoID == 0 {
		return fmt.Errorf("無效的分析結果或 VideoID 為空")
	}
	query := `
		INSERT INTO analysis_results (
			video_id, transcript, translation, short_summary, bulleted_summary, bites, 
			mentioned_locations, importance_score, material_type, related_news,
			visual_description, topics, keywords, error_message, prompt_version, 
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			transcript = VALUES(transcript), translation = VALUES(translation), short_summary = VALUES(short_summary),
			bulleted_summary = VALUES(bulleted_summary), bites = VALUES(bites),
			mentioned_locations = VALUES(mentioned_locations), importance_score = VALUES(importance_score),
			material_type = VALUES(material_type), related_news = VALUES(related_news),
			visual_description = VALUES(visual_description), topics = VALUES(topics), 
			keywords = VALUES(keywords), error_message = VALUES(error_message), 
			prompt_version = VALUES(prompt_version), updated_at = VALUES(updated_at);`

	toSQLNullString := func(jns *models.JsonNullString) sql.NullString {
		if jns != nil {
			return jns.NullString
		}
		return sql.NullString{Valid: false}
	}

	var promptVersion sql.NullString
	if result.PromptVersion != "" {
		promptVersion = sql.NullString{String: result.PromptVersion, Valid: true}
	}

	// 確保 CreatedAt 和 UpdatedAt 有值
	createdAt := result.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt := result.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	_, err := s.db.Exec(query,
		result.VideoID,
		toSQLNullString(result.Transcript),
		toSQLNullString(result.Translation),
		toSQLNullString(result.ShortSummary),
		toSQLNullString(result.BulletedSummary),
		result.Bites,              // json.RawMessage (可以是 nil)
		result.MentionedLocations, // json.RawMessage (可以是 nil)
		result.ImportanceScore,    // json.RawMessage (可以是 nil)
		toSQLNullString(result.MaterialType),
		result.RelatedNews, // json.RawMessage (可以是 nil)
		toSQLNullString(result.VisualDescription),
		result.Topics,   // json.RawMessage (可以是 nil)
		result.Keywords, // json.RawMessage (可以是 nil)
		toSQLNullString(result.ErrorMessage),
		promptVersion,
		createdAt,
		updatedAt,
	)
	if err != nil {
		return fmt.Errorf("儲存分析結果到資料庫失敗 (VideoID: %d): %w", result.VideoID, err)
	}
	log.Printf("資訊：分析結果成功儲存到資料庫 (VideoID: %d, PromptVersion: %s)\n", result.VideoID, result.PromptVersion)
	return nil
}

// UpdateVideoAnalysisStatus 更新影片的分析狀態、分析時間
func (s *MySQLStore) UpdateVideoAnalysisStatus(videoID int64, status models.AnalysisStatus, analyzedAt sql.NullTime, errorMessage sql.NullString) error {
	if videoID == 0 {
		return fmt.Errorf("無效的 VideoID")
	}
	// errorMessage 應記錄在 analysis_results 表中，此方法專注更新 videos 表的狀態
	query := "UPDATE videos SET analysis_status = ?, analyzed_at = ? WHERE id = ?"
	params := []interface{}{status, analyzedAt, videoID}

	_, err := s.db.Exec(query, params...)
	if err != nil {
		return fmt.Errorf("更新影片分析狀態失敗 (VideoID: %d, Status: %s): %w", videoID, status, err)
	}
	log.Printf("資訊：影片分析狀態成功更新 (VideoID: %d, Status: %s)\n", videoID, status)
	return nil
}

// GetPendingVideos (與之前版本相同，確保SELECT和Scan包含所有欄位)
func (s *MySQLStore) GetPendingVideos(limit int) ([]models.Video, error) {
	query := `
		SELECT id, source_name, source_id, nas_path, title, 
		       fetched_at, published_at, duration_secs, shotlist_content, view_link, 
		       subjects, location, analysis_status, analyzed_at, source_metadata
		FROM videos
		WHERE analysis_status = ? OR analysis_status = ? OR analysis_status = ?
		ORDER BY fetched_at ASC
		LIMIT ?;`
	rows, err := s.db.Query(query, models.StatusPending, models.StatusTxtAnalysisFailed, models.StatusMetadataExtracted, limit)
	if err != nil {
		return nil, fmt.Errorf("查詢待處理影片失敗: %w", err)
	}
	defer rows.Close()
	var videos []models.Video
	for rows.Next() {
		var v models.Video
		var sourceMetadataSQL, subjectsSQL sql.RawBytes
		var shotlistContentSQL, viewLinkSQL, locationSQL sql.NullString
		err := rows.Scan(
			&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title,
			&v.FetchedAt, &v.PublishedAt, &v.DurationSecs, &shotlistContentSQL, &viewLinkSQL,
			&subjectsSQL, &locationSQL,
			&v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataSQL,
		)
		if err != nil {
			log.Printf("錯誤：掃描待處理影片查詢結果行失敗: %v", err)
			continue
		}
		if sourceMetadataSQL != nil {
			v.SourceMetadata = json.RawMessage(sourceMetadataSQL)
		}
		if subjectsSQL != nil {
			v.Subjects = json.RawMessage(subjectsSQL)
		}
		v.ShotlistContent = models.JsonNullString{NullString: shotlistContentSQL}
		v.Location = locationSQL
		if viewLinkSQL.Valid {
			v.ViewLink = viewLinkSQL
		}
		videos = append(videos, v)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("處理待處理影片查詢結果集時發生錯誤: %w", err)
	}
	log.Printf("資訊：查詢到 %d 個待處理影片。\n", len(videos))
	return videos, nil
}

// GetVideoByID (與之前版本相同，確保SELECT和Scan包含所有欄位)
func (s *MySQLStore) GetVideoByID(videoID int64) (*models.Video, error) {
	if videoID == 0 {
		return nil, fmt.Errorf("無效的 VideoID")
	}
	query := `
		SELECT id, source_name, source_id, nas_path, title, 
		       fetched_at, published_at, duration_secs, shotlist_content, view_link, 
		       subjects, location, analysis_status, analyzed_at, source_metadata
		FROM videos
		WHERE id = ?;`
	row := s.db.QueryRow(query, videoID)
	var v models.Video
	var sourceMetadataBytes, subjectsBytes []byte
	var shotlistContentSQL, locationSQL, viewLinkSQL sql.NullString
	err := row.Scan(
		&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title,
		&v.FetchedAt, &v.PublishedAt, &v.DurationSecs, &shotlistContentSQL, &viewLinkSQL,
		&subjectsBytes, &locationSQL,
		&v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataBytes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查詢 VideoID %d 失敗: %w", videoID, err)
	}
	if sourceMetadataBytes != nil {
		v.SourceMetadata = json.RawMessage(sourceMetadataBytes)
	}
	if subjectsBytes != nil {
		v.Subjects = json.RawMessage(subjectsBytes)
	}
	v.ShotlistContent = models.JsonNullString{NullString: shotlistContentSQL}
	v.Location = locationSQL
	if viewLinkSQL.Valid {
		v.ViewLink = viewLinkSQL
	}
	return &v, nil
}

// GetVideosPendingContentAnalysis (與之前版本相同，確保SELECT和Scan包含所有欄位)
func (s *MySQLStore) GetVideosPendingContentAnalysis(status models.AnalysisStatus, limit int) ([]models.Video, error) {
	query := `
		SELECT id, source_name, source_id, nas_path, title, 
		       fetched_at, published_at, duration_secs, shotlist_content, view_link, 
		       subjects, location, analysis_status, analyzed_at, source_metadata
		FROM videos
		WHERE analysis_status = ?
		ORDER BY fetched_at ASC
		LIMIT ?;`
	rows, err := s.db.Query(query, status, limit)
	if err != nil {
		return nil, fmt.Errorf("查詢狀態為 '%s' 的影片失敗: %w", status, err)
	}
	defer rows.Close()
	var videos []models.Video
	for rows.Next() {
		var v models.Video
		var sourceMetadataSQL, subjectsSQL sql.RawBytes
		var shotlistContentSQL, viewLinkSQL, locationSQL sql.NullString
		err := rows.Scan(
			&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title,
			&v.FetchedAt, &v.PublishedAt, &v.DurationSecs, &shotlistContentSQL, &viewLinkSQL,
			&subjectsSQL, &locationSQL,
			&v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataSQL,
		)
		if err != nil {
			log.Printf("錯誤：掃描狀態為 '%s' 的影片查詢結果行失敗: %v", status, err)
			continue
		}
		if sourceMetadataSQL != nil {
			v.SourceMetadata = json.RawMessage(sourceMetadataSQL)
		}
		if subjectsSQL != nil {
			v.Subjects = json.RawMessage(subjectsSQL)
		}
		v.ShotlistContent = models.JsonNullString{NullString: shotlistContentSQL}
		v.Location = locationSQL
		if viewLinkSQL.Valid {
			v.ViewLink = viewLinkSQL
		}
		videos = append(videos, v)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("處理狀態為 '%s' 的影片查詢結果集時發生錯誤: %w", status, err)
	}
	log.Printf("資訊：查詢到 %d 個狀態為 '%s' 的影片。\n", len(videos), status)
	return videos, nil
}
