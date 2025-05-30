package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// AnalysisStatus 定義分析狀態
type AnalysisStatus string

const (
	StatusPending             AnalysisStatus = "pending"               // 初始狀態，或等待文本元數據分析
	StatusMetadataExtracting  AnalysisStatus = "metadata_extracting"   // 新增：正在提取文本元數據
	StatusMetadataExtracted   AnalysisStatus = "metadata_extracted"    // 新增：文本元數據已提取，等待影片內容分析
	StatusProcessing          AnalysisStatus = "processing"            // 正在進行影片內容分析
	StatusCompleted           AnalysisStatus = "completed"             // 所有分析已完成
	StatusFailed              AnalysisStatus = "failed"                // 任一階段分析失敗
	StatusTxtAnalysisFailed   AnalysisStatus = "txt_analysis_failed"   // 新增：文本分析失敗
	StatusVideoAnalysisFailed AnalysisStatus = "video_analysis_failed" // 新增：影片內容分析失敗 (取代通用的 failed)
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

// ParsedTxtData (保持不變)
type ParsedTxtData struct {
	Title           string          `json:"title"`
	CreationDateStr string          `json:"creation_date"`
	DurationSeconds json.RawMessage `json:"duration_seconds"` // <--- 修改為 json.RawMessage
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
	ShotlistContent JsonNullString  `json:"shotlist_content"`
	ViewLink        sql.NullString  `json:"view_link"`
	Subjects        json.RawMessage `json:"subjects"`
	Location        sql.NullString  `json:"location"`
	AnalysisStatus  AnalysisStatus  `json:"analysis_status"`
	AnalyzedAt      sql.NullTime    `json:"analyzed_at"`
	SourceMetadata  json.RawMessage `json:"source_metadata"`
}
