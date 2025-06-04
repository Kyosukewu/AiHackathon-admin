package mysql

import (
	"AiHackathon-admin/internal/config"
	"AiHackathon-admin/internal/models"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLStore 結構
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore, Close, copyBytes, appendIfMissingVideo (保持不變)
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
		return nil, fmt.Errorf("無法連線到資料庫 (ping 失敗): %w", err)
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
func copyBytes(src []byte) []byte {
	if src == nil {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
func appendIfMissingVideo(slice []models.Video, v models.Video) []models.Video {
	for _, existing := range slice {
		if existing.ID == v.ID {
			return slice
		}
	}
	return append(slice, v)
}

// GetAllVideosWithAnalysis (保持不變)
func (s *MySQLStore) GetAllVideosWithAnalysis(limit int, offset int, searchTerm string, sortBy string, sortOrder string) ([]models.Video, []models.AnalysisResult, error) {
	log.Printf("資訊：MySQLStore.GetAllVideosWithAnalysis 被呼叫 (limit: %d, offset: %d, search: '%s', sortBy: '%s', sortOrder: '%s')\n", limit, offset, searchTerm, sortBy, sortOrder)
	var args []interface{}
	baseQuery := `
		SELECT
			v.id, v.source_name, v.source_id, v.nas_path, v.title, 
			v.fetched_at, v.published_at, v.duration_secs, v.shotlist_content, v.view_link,
			v.analysis_status, v.analyzed_at, v.source_metadata,
			v.subjects, v.location, v.restrictions, v.tran_restrictions,
			v.prompt_version,
			ar.video_id, ar.transcript, ar.translation, 
			ar.short_summary, ar.bulleted_summary, ar.bites, ar.mentioned_locations,
			ar.importance_score, ar.material_type, ar.related_news,
			ar.visual_description, ar.topics, ar.keywords, ar.error_message,
			ar.prompt_version, ar.created_at, ar.updated_at
		FROM videos v
		LEFT JOIN analysis_results ar ON v.id = ar.video_id
	`
	whereClauses := []string{}
	if searchTerm != "" {
		likeTerm := "%" + strings.ReplaceAll(strings.ReplaceAll(searchTerm, "%", "\\%"), "_", "\\_") + "%"
		searchFieldsClause := `(
			v.source_id LIKE ? OR v.title LIKE ? OR IFNULL(v.shotlist_content, '') LIKE ? OR 
			IFNULL(ar.transcript, '') LIKE ? OR IFNULL(ar.short_summary, '') LIKE ? OR 
			IFNULL(ar.bulleted_summary, '') LIKE ? OR IFNULL(ar.keywords, '') LIKE ? OR 
			IFNULL(ar.visual_description, '') LIKE ? OR v.id LIKE ? 
		)`
		whereClauses = append(whereClauses, searchFieldsClause)
		for i := 0; i < 9; i++ {
			args = append(args, likeTerm)
		}
	}
	// 新增：若 sortOrder 為合法 analysis_status，則加上狀態過濾
	validStatuses := map[string]bool{
		string(models.StatusPending):             true,
		string(models.StatusMetadataExtracting):  true,
		string(models.StatusMetadataExtracted):   true,
		string(models.StatusTxtAnalysisFailed):   true,
		string(models.StatusProcessing):          true,
		string(models.StatusVideoAnalysisFailed): true,
		string(models.StatusCompleted):           true,
		string(models.StatusFailed):              true,
	}
	if validStatuses[sortOrder] {
		whereClauses = append(whereClauses, "v.analysis_status = ?")
		args = append(args, sortOrder)
	}
	if len(whereClauses) > 0 {
		baseQuery += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	orderByClause := "ORDER BY v.fetched_at DESC"
	if sortBy != "" && sortBy != "importance" {
		column := "v.fetched_at"
		switch sortBy {
		case "published_at":
			column = "v.published_at"
		case "source_id":
			column = "v.source_id"
		}
		orderDirection := "DESC"
		if strings.ToLower(sortOrder) == "asc" {
			orderDirection = "ASC"
		}
		orderByClause = fmt.Sprintf("ORDER BY %s %s, v.id %s", column, orderDirection, orderDirection)
	}
	baseQuery += " " + orderByClause
	baseQuery += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	log.Printf("DEBUG SQL Query (GetAllVideosWithAnalysis): %s\nArgs: %v\n", baseQuery, args)

	// 新增 debug log：印出所有影片的 id 和 analysis_status
	debugQuery := "SELECT id, analysis_status FROM videos"
	debugRows, err := s.db.Query(debugQuery)
	if err != nil {
		log.Printf("錯誤：查詢所有影片狀態失敗: %v", err)
	} else {
		defer debugRows.Close()
		log.Println("DEBUG 所有影片狀態:")
		for debugRows.Next() {
			var id int64
			var status string
			if err := debugRows.Scan(&id, &status); err != nil {
				log.Printf("錯誤：掃描影片狀態失敗: %v", err)
				continue
			}
			log.Printf("影片 ID: %d, 狀態: %s", id, status)
		}
		if err = debugRows.Err(); err != nil {
			log.Printf("錯誤：處理影片狀態查詢結果集時發生錯誤: %v", err)
		}
	}

	rows, err := s.db.Query(baseQuery, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("查詢影片和分析結果失敗: %w", err)
	}
	defer rows.Close()

	videosMap := make(map[int64]models.Video)
	analysisResultMap := make(map[int64]models.AnalysisResult)

	for rows.Next() {
		var v models.Video
		var arTemp models.AnalysisResult
		var sourceMetadataSQL, subjectsSQL sql.RawBytes
		var shotlistContentSQL, viewLinkSQL, locationSQL, restrictionsSQL, tranRestrictionsSQL sql.NullString
		var arVideoID sql.NullInt64
		var arTranscriptSQL, arTranslationSQL, arShortSummarySQL, arBulletedSummarySQL, arMaterialTypeSQL, arVisualDescriptionSQL, arErrorMessageSQL, arPromptVersionSQL sql.NullString
		var arTopicsSQL, arKeywordsSQL, arBitesSQL, arMentionedLocationsSQL, arImportanceScoreSQL, arRelatedNewsSQL sql.RawBytes
		var arCreatedAt, arUpdatedAt sql.NullTime

		scanTargets := []interface{}{
			&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title,
			&v.FetchedAt, &v.PublishedAt, &v.DurationSecs, &shotlistContentSQL, &viewLinkSQL,
			&v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataSQL,
			&subjectsSQL, &locationSQL, &restrictionsSQL, &tranRestrictionsSQL, &v.PromptVersion,
			&arVideoID, &arTranscriptSQL, &arTranslationSQL, &arShortSummarySQL, &arBulletedSummarySQL,
			&arBitesSQL, &arMentionedLocationsSQL, &arImportanceScoreSQL, &arMaterialTypeSQL, &arRelatedNewsSQL,
			&arVisualDescriptionSQL, &arTopicsSQL, &arKeywordsSQL, &arErrorMessageSQL, &arPromptVersionSQL,
			&arCreatedAt, &arUpdatedAt,
		}
		if err := rows.Scan(scanTargets...); err != nil {
			log.Printf("錯誤：[GetAllVideos] 掃描查詢結果行失敗: %v", err)
			continue
		}

		if _, ok := videosMap[v.ID]; !ok {
			if sourceMetadataSQL != nil {
				v.SourceMetadata = copyBytes(sourceMetadataSQL)
			} else {
				v.SourceMetadata = nil
			}
			if subjectsSQL != nil {
				v.Subjects = copyBytes(subjectsSQL)
				log.Printf("DEBUG DB GETALL: VideoID %d, Scanned Subjects (copied): %s", v.ID, string(v.Subjects))
			} else {
				v.Subjects = nil
			}
			v.ShotlistContent = models.JsonNullString{NullString: shotlistContentSQL}
			v.Location = locationSQL
			v.Restrictions = restrictionsSQL
			v.TranRestrictions = tranRestrictionsSQL
			if viewLinkSQL.Valid {
				v.ViewLink = viewLinkSQL
			}
			videosMap[v.ID] = v
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
				arTemp.Bites = copyBytes(arBitesSQL)
			} else {
				arTemp.Bites = nil
			}
			if arMentionedLocationsSQL != nil {
				arTemp.MentionedLocations = copyBytes(arMentionedLocationsSQL)
			} else {
				arTemp.MentionedLocations = nil
			}
			if arImportanceScoreSQL != nil {
				arTemp.ImportanceScore = copyBytes(arImportanceScoreSQL)
			} else {
				arTemp.ImportanceScore = nil
			}
			if arRelatedNewsSQL != nil {
				arTemp.RelatedNews = copyBytes(arRelatedNewsSQL)
			} else {
				arTemp.RelatedNews = nil
			}
			if arTopicsSQL != nil {
				arTemp.Topics = copyBytes(arTopicsSQL)
			} else {
				arTemp.Topics = nil
			}
			if arKeywordsSQL != nil {
				arTemp.Keywords = copyBytes(arKeywordsSQL)
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

	var finalVideos []models.Video
	for _, v := range videosMap {
		finalVideos = append(finalVideos, v)
	}
	sort.SliceStable(finalVideos, func(i, j int) bool { return finalVideos[i].ID < finalVideos[j].ID })

	var finalAnalysisResults []models.AnalysisResult
	for _, v_ := range finalVideos {
		if ar, ok := analysisResultMap[v_.ID]; ok {
			finalAnalysisResults = append(finalAnalysisResults, ar)
		}
	}
	log.Printf("資訊：查詢到 %d 個影片，%d 個有效分析結果。\n", len(finalVideos), len(finalAnalysisResults))
	return finalVideos, finalAnalysisResults, nil
}

// FindOrCreateVideo (保持不變)
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
	} else if video.NASPath != "" {
		query := "SELECT id FROM videos WHERE nas_path = ?"
		queryErr = s.db.QueryRow(query, video.NASPath).Scan(&videoID)
	} else {
		return 0, fmt.Errorf("無法確定查找影片的唯一標識")
	}
	if queryErr == sql.ErrNoRows {
		log.Printf("資訊：資料庫中未找到影片 (Source: %s, ID: %s, NAS: %s)，正在新增記錄...\n", video.SourceName, video.SourceID, video.NASPath)
		log.Printf("DEBUG DB INSERT: Video.Subjects to be inserted: '%s', AnalysisStatus: '%s', AnalyzedAt: %v", string(video.Subjects), video.AnalysisStatus, video.AnalyzedAt)
		insertQuery := ` INSERT INTO videos ( source_name, source_id, nas_path, title, fetched_at, published_at, duration_secs, shotlist_content, view_link, subjects, location, restrictions, tran_restrictions, analysis_status, source_metadata, analyzed_at ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
		fetchedTime := video.FetchedAt
		if fetchedTime.IsZero() {
			fetchedTime = time.Now()
		}
		status := video.AnalysisStatus
		if status == "" {
			status = models.StatusPending
		}
		res, insertErr := s.db.Exec(insertQuery, video.SourceName, video.SourceID, video.NASPath, video.Title, fetchedTime, video.PublishedAt, video.DurationSecs, video.ShotlistContent, video.ViewLink, video.Subjects, video.Location, video.Restrictions, video.TranRestrictions, status, video.SourceMetadata, video.AnalyzedAt)
		if insertErr != nil {
			return 0, fmt.Errorf("插入新影片記錄失敗 (Source: %s, ID: %s): %w", video.SourceName, video.SourceID, insertErr)
		}
		videoID, insertErr = res.LastInsertId()
		if insertErr != nil {
			return 0, fmt.Errorf("獲取新插入影片的 ID 失敗 (Source: %s, ID: %s): %w", video.SourceName, video.SourceID, insertErr)
		}
		log.Printf("資訊：新增影片記錄成功，ID: %d (Source: %s, ID: %s)\n", videoID, video.SourceName, video.SourceID)
		return videoID, nil
	} else if queryErr != nil {
		return 0, fmt.Errorf("查找影片失敗 (Source: %s, ID: %s): %w", video.SourceName, video.SourceID, queryErr)
	}
	log.Printf("資訊：資料庫中已存在影片記錄 ID: %d (Source: %s, ID: %s)。正在更新元數據...\n", videoID, video.SourceName, video.SourceID)
	log.Printf("DEBUG DB UPDATE for VideoID %d: Subjects to be updated: '%s', AnalysisStatus: '%s', AnalyzedAt: '%v'", videoID, string(video.Subjects), video.AnalysisStatus, video.AnalyzedAt)

	// 確保 analysis_status 不為空
	status := video.AnalysisStatus
	if status == "" {
		status = models.StatusPending
	}

	updateQuery := ` UPDATE videos SET title = ?, published_at = ?, duration_secs = ?, shotlist_content = ?, view_link = ?, subjects = ?, location = ?, restrictions = ?, tran_restrictions = ?, nas_path = ?, source_metadata = ?, fetched_at = ?, analysis_status = ?, analyzed_at = ?, prompt_version = ? WHERE id = ?;`
	_, updateErr := s.db.Exec(updateQuery, video.Title, video.PublishedAt, video.DurationSecs, video.ShotlistContent, video.ViewLink, video.Subjects, video.Location, video.Restrictions, video.TranRestrictions, video.NASPath, video.SourceMetadata, video.FetchedAt, status, video.AnalyzedAt, video.PromptVersion, videoID)
	if updateErr != nil {
		return 0, fmt.Errorf("更新影片 ID %d 的元數據失敗: %w", videoID, updateErr)
	}
	log.Printf("資訊：影片 ID %d 的元數據更新成功 (狀態更新為: %s)。\n", videoID, status)
	return videoID, nil
}

// SaveAnalysisResult (增加詳細日誌)
func (s *MySQLStore) SaveAnalysisResult(result *models.AnalysisResult) error {
	if result == nil || result.VideoID == 0 {
		return fmt.Errorf("無效的分析結果或 VideoID 為空")
	}
	// *** 新增日誌：記錄準備儲存的 AnalysisResult 的所有 JSON 相關欄位 ***
	log.Printf("DEBUG DB SAVE_ANALYSIS: VideoID: %d, PromptVersion: %s, TranscriptValid: %t, TranslationValid: %t, ShortSummaryValid: %t, BulletedSummaryValid: %t, VisualDescriptionValid: %t, MaterialTypeValid: %t, ErrorMessageValid: %t",
		result.VideoID, result.PromptVersion,
		result.Transcript != nil && result.Transcript.Valid,
		result.Translation != nil && result.Translation.Valid,
		result.ShortSummary != nil && result.ShortSummary.Valid,
		result.BulletedSummary != nil && result.BulletedSummary.Valid,
		result.VisualDescription != nil && result.VisualDescription.Valid,
		result.MaterialType != nil && result.MaterialType.Valid,
		result.ErrorMessage != nil && result.ErrorMessage.Valid,
	)
	log.Printf("DEBUG DB SAVE_ANALYSIS (JSON fields for VideoID: %d): Topics: '%s', Keywords: '%s', Bites: '%s', Locations: '%s', Importance: '%s', RelatedNews: '%s'",
		result.VideoID, string(result.Topics), string(result.Keywords), string(result.Bites),
		string(result.MentionedLocations), string(result.ImportanceScore), string(result.RelatedNews))
	// *** 結束新增日誌 ***

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
			return jns.NullString // models.JsonNullString 嵌入了 sql.NullString
		}
		return sql.NullString{Valid: false}
	}

	var promptVersion sql.NullString
	if result.PromptVersion != "" {
		promptVersion = sql.NullString{String: result.PromptVersion, Valid: true}
	}

	createdAt := result.CreatedAt
	if createdAt.IsZero() { // 如果服務層沒有設定，則使用當前時間
		createdAt = time.Now()
	}
	updatedAt := result.UpdatedAt
	if updatedAt.IsZero() { // 如果服務層沒有設定，則使用當前時間
		updatedAt = time.Now()
	}

	_, err := s.db.Exec(query,
		result.VideoID,
		toSQLNullString(result.Transcript),
		toSQLNullString(result.Translation),
		toSQLNullString(result.ShortSummary),
		toSQLNullString(result.BulletedSummary),
		result.Bites,              // json.RawMessage
		result.MentionedLocations, // json.RawMessage
		result.ImportanceScore,    // json.RawMessage
		toSQLNullString(result.MaterialType),
		result.RelatedNews, // json.RawMessage
		toSQLNullString(result.VisualDescription),
		result.Topics,   // json.RawMessage
		result.Keywords, // json.RawMessage
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
func (s *MySQLStore) UpdateVideoAnalysisStatus(videoID int64, status models.AnalysisStatus, analyzedAt sql.NullTime, errorMessage sql.NullString) error { /* ... */
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
func (s *MySQLStore) GetPendingVideos(limit int) ([]models.Video, error) {
	query := ` SELECT id, source_name, source_id, nas_path, title, fetched_at, published_at, duration_secs, shotlist_content, view_link, subjects, location, analysis_status, analyzed_at, source_metadata FROM videos WHERE analysis_status = ? OR analysis_status = ? OR analysis_status = ? ORDER BY fetched_at ASC LIMIT ?;`
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
		err := rows.Scan(&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title, &v.FetchedAt, &v.PublishedAt, &v.DurationSecs, &shotlistContentSQL, &viewLinkSQL, &subjectsSQL, &locationSQL, &v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataSQL)
		if err != nil {
			log.Printf("錯誤：掃描待處理影片查詢結果行失敗: %v", err)
			continue
		}
		if sourceMetadataSQL != nil {
			v.SourceMetadata = copyBytes(sourceMetadataSQL)
		}
		if subjectsSQL != nil {
			v.Subjects = copyBytes(subjectsSQL)
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
func (s *MySQLStore) GetVideoByID(videoID int64) (*models.Video, error) {
	if videoID == 0 {
		return nil, fmt.Errorf("無效的 VideoID")
	}
	query := ` SELECT id, source_name, source_id, nas_path, title, fetched_at, published_at, duration_secs, shotlist_content, view_link, subjects, location, analysis_status, analyzed_at, source_metadata FROM videos WHERE id = ?;`
	row := s.db.QueryRow(query, videoID)
	var v models.Video
	var sourceMetadataBytes, subjectsBytes []byte
	var shotlistContentSQL, locationSQL, viewLinkSQL sql.NullString
	err := row.Scan(&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title, &v.FetchedAt, &v.PublishedAt, &v.DurationSecs, &shotlistContentSQL, &viewLinkSQL, &subjectsBytes, &locationSQL, &v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查詢 VideoID %d 失敗: %w", videoID, err)
	}
	if sourceMetadataBytes != nil {
		v.SourceMetadata = copyBytes(sourceMetadataBytes)
	}
	if subjectsBytes != nil {
		v.Subjects = copyBytes(subjectsBytes)
	}
	v.ShotlistContent = models.JsonNullString{NullString: shotlistContentSQL}
	v.Location = locationSQL
	if viewLinkSQL.Valid {
		v.ViewLink = viewLinkSQL
	}
	return &v, nil
}
func (s *MySQLStore) GetVideosPendingContentAnalysis(status models.AnalysisStatus, limit int) ([]models.Video, error) {
	query := ` SELECT id, source_name, source_id, nas_path, title, fetched_at, published_at, duration_secs, shotlist_content, view_link, subjects, location, analysis_status, analyzed_at, source_metadata FROM videos WHERE analysis_status = ? ORDER BY fetched_at ASC LIMIT ?;`
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
		err := rows.Scan(&v.ID, &v.SourceName, &v.SourceID, &v.NASPath, &v.Title, &v.FetchedAt, &v.PublishedAt, &v.DurationSecs, &shotlistContentSQL, &viewLinkSQL, &subjectsSQL, &locationSQL, &v.AnalysisStatus, &v.AnalyzedAt, &sourceMetadataSQL)
		if err != nil {
			log.Printf("錯誤：掃描狀態為 '%s' 的影片查詢結果行失敗: %v", status, err)
			continue
		}
		if sourceMetadataSQL != nil {
			v.SourceMetadata = copyBytes(sourceMetadataSQL)
		}
		if subjectsSQL != nil {
			v.Subjects = copyBytes(subjectsSQL)
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
