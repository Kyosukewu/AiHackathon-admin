// AiHackathon-admin/internal/models/analysis.go
package models

import (
	"encoding/json"
	"time"
	// "AiHackathon-admin/internal/models/types" // 如果 JsonNullString 在 types.go 且 types.go 是 package models，則不需此行
)

// AnalysisResult 對應 analysis_results 資料表
type AnalysisResult struct {
	VideoID            int64           `json:"-"`
	Transcript         *JsonNullString `json:"transcript,omitempty"`         // 來自 types.go 或同 package
	Translation        *JsonNullString `json:"translation,omitempty"`        // 來自 types.go 或同 package
	VisualDescription  *JsonNullString `json:"visual_description,omitempty"` // 來自 types.go 或同 package
	ShortSummary       *JsonNullString `json:"short_summary,omitempty"`      // 來自 types.go 或同 package
	BulletedSummary    *JsonNullString `json:"bulleted_summary,omitempty"`   // 來自 types.go 或同 package
	Bites              json.RawMessage `json:"bites,omitempty"`
	MentionedLocations json.RawMessage `json:"mentioned_locations,omitempty"`
	ImportanceScore    json.RawMessage `json:"importance_score,omitempty"`
	MaterialType       *JsonNullString `json:"material_type,omitempty"` // 來自 types.go 或同 package
	RelatedNews        json.RawMessage `json:"related_news,omitempty"`
	Topics             json.RawMessage `json:"topics,omitempty"`
	Keywords           json.RawMessage `json:"keywords,omitempty"`
	ErrorMessage       *JsonNullString `json:"error_message,omitempty"` // 來自 types.go 或同 package
	PromptVersion      string          `json:"-"`
	CreatedAt          time.Time       `json:"-"`
	UpdatedAt          time.Time       `json:"-"`
}
