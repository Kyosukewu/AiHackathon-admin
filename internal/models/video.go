package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// AnalysisStatus 定義分析狀態 (保持不變)
type AnalysisStatus string

const (
	StatusPending    AnalysisStatus = "pending"
	StatusProcessing AnalysisStatus = "processing"
	StatusCompleted  AnalysisStatus = "completed"
	StatusFailed     AnalysisStatus = "failed"
)

// Video 對應 videos 資料表 (保持不變)
type Video struct {
	ID             int64           `json:"id"`
	SourceName     string          `json:"source_name"`
	SourceID       string          `json:"source_id"`
	NASPath        string          `json:"nas_path"`
	Title          sql.NullString  `json:"title"`
	FetchedAt      time.Time       `json:"fetched_at"`
	AnalysisStatus AnalysisStatus  `json:"analysis_status"`
	AnalyzedAt     sql.NullTime    `json:"analyzed_at"`
	SourceMetadata json.RawMessage `json:"source_metadata"`
}

// --- 新增 VideoFileInfo 結構 ---
// VideoFileInfo 用於儲存從檔案系統掃描到的影片檔案資訊
type VideoFileInfo struct {
	AbsolutePath string    // 影片在 NAS 上的絕對路徑
	RelativePath string    // 相對於 NAS basePath 的路徑 (用於存入 DB)
	SourceName   string    // 從目錄結構解析出的來源名稱
	OriginalID   string    // 從目錄結構解析出的原始 ID (如果有的話)
	FileName     string    // 檔名
	ModTime      time.Time // 檔案的最後修改時間
}

// --- 結束新增 ---
