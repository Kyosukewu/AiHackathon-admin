package models

import (
	"encoding/json"
	"time"
	// "database/sql" // JsonNullString 來自 types.go
)

// AnalysisResult 對應 analysis_results 資料表
// 更新欄位類型以使用自訂的 *JsonNullString
type AnalysisResult struct {
	VideoID           int64           `json:"-"`
	Transcript        *JsonNullString `json:"transcript,omitempty"`         // 改為指標
	Translation       *JsonNullString `json:"translation,omitempty"`        // 改為指標
	Summary           *JsonNullString `json:"summary,omitempty"`            // 改為指標
	VisualDescription *JsonNullString `json:"visual_description,omitempty"` // 改為指標
	Topics            json.RawMessage `json:"topics,omitempty"`             // json.RawMessage 可以處理 null
	Keywords          json.RawMessage `json:"keywords,omitempty"`           // json.RawMessage 可以處理 null
	ErrorMessage      *JsonNullString `json:"error_message,omitempty"`      // 改為指標
	PromptVersion     string          `json:"-"`
	CreatedAt         time.Time       `json:"-"`
	UpdatedAt         time.Time       `json:"-"`
}
