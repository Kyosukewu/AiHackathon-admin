package models

import (
	// "AiHackathon-admin/internal/models/types" // 如果 types.go 和本檔案在同一個 package models，則不需此行
	"database/sql"
	"encoding/json"
	"time"
)

// AnalysisStatus (保持不變)
type AnalysisStatus string

const (
	StatusPending             AnalysisStatus = "pending"
	StatusMetadataExtracting  AnalysisStatus = "metadata_extracting"
	StatusMetadataExtracted   AnalysisStatus = "metadata_extracted"
	StatusTxtAnalysisFailed   AnalysisStatus = "txt_analysis_failed"
	StatusProcessing          AnalysisStatus = "processing"
	StatusVideoAnalysisFailed AnalysisStatus = "video_analysis_failed"
	StatusCompleted           AnalysisStatus = "completed"
	StatusFailed              AnalysisStatus = "failed" // 通用失敗，可考慮是否保留
)

// VideoFileInfo (保持不變)
type VideoFileInfo struct {
	VideoAbsolutePath string
	TextFilePath      string
	RelativePath      string
	SourceName        string
	OriginalID        string
	VideoFileName     string
	ModTime           time.Time
}

// ParsedTxtData 用於存放從 Gemini 分析 .txt 檔案後回傳的 JSON 數據
type ParsedTxtData struct {
	Title           string          `json:"title"`
	CreationDateStr string          `json:"creation_date"`
	DurationSeconds json.RawMessage `json:"duration_seconds"` // <--- 已改回 json.RawMessage
	Subjects        json.RawMessage `json:"subjects"`
	Location        string          `json:"location"`
	ShotlistContent string          `json:"shotlist_content"`
}

// Video 結構 (保持不變)
type Video struct {
	ID              int64           `json:"id"`
	SourceName      string          `json:"source_name"`
	SourceID        string          `json:"source_id"`
	NASPath         string          `json:"nas_path"`
	Title           sql.NullString  `json:"title"`
	FetchedAt       time.Time       `json:"fetched_at"`
	PublishedAt     sql.NullTime    `json:"published_at"`
	DurationSecs    sql.NullInt64   `json:"duration_secs"`
	ShotlistContent JsonNullString  `json:"shotlist_content"` // 來自 types.go (同 package)
	ViewLink        sql.NullString  `json:"view_link"`
	Subjects        json.RawMessage `json:"subjects"`
	Location        sql.NullString  `json:"location"`
	AnalysisStatus  AnalysisStatus  `json:"analysis_status"`
	AnalyzedAt      sql.NullTime    `json:"analyzed_at"`
	SourceMetadata  json.RawMessage `json:"source_metadata"`
}
