package handlers

import (
	"AiHackathon-admin/internal/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
)

// DBStore 定義了應用程式需要的資料庫操作介面
type DBStore interface {
	GetAllVideosWithAnalysis(limit int, offset int) ([]models.Video, []models.AnalysisResult, error)
	Close() error
	FindOrCreateVideo(video *models.Video) (int64, error)
	SaveAnalysisResult(result *models.AnalysisResult) error
	UpdateVideoAnalysisStatus(videoID int64, status models.AnalysisStatus, analyzedAt sql.NullTime, errorMessage sql.NullString) error
	GetPendingVideos(limit int) ([]models.Video, error)
	GetVideoByID(videoID int64) (*models.Video, error)
	GetVideosPendingContentAnalysis(status models.AnalysisStatus, limit int) ([]models.Video, error)
}

// DashboardPageData 用於傳遞給 HTML 範本的數據
type DashboardPageData struct {
	Videos []VideoDisplayData
}

// VideoDisplayData 用於在範本中顯示的影片數據，包含格式化後的欄位
type VideoDisplayData struct {
	VideoID                  int64
	SourceName               string
	SourceID                 string
	NASPath                  string
	Title                    string
	AnalysisStatus           models.AnalysisStatus
	AnalysisResult           *DisplayableAnalysisResult
	PublishedAt              sql.NullTime
	DurationSecs             sql.NullInt64
	FormattedDurationMinutes int64
	FormattedDurationSeconds int64
	ShotlistContent          models.JsonNullString
	ViewLink                 sql.NullString
	PrimaryLocation          string
	PrimarySubjects          []string
	FlagEmoji                string
	VideoURL                 string
}

// KeywordDisplay 用於在範本中顯示關鍵詞及其分類
type KeywordDisplay struct {
	Keyword  string `json:"keyword"`
	Category string `json:"category"`
}

// BiteDisplay 用於顯示 BITE 的結構
type BiteDisplay struct {
	Speaker string `json:"speaker"`
	Quote   string `json:"quote"`
}

// ImportanceScoreDisplay 用於顯示重要性評分的結構
type ImportanceScoreDisplay struct {
	OverallRating     string   `json:"overall_rating"`
	KeyFactors        []string `json:"key_factors"`
	AssessmentDetails string   `json:"assessment_details"`
}

// DisplayableAnalysisResult 用於在範本中顯示的分析結果
type DisplayableAnalysisResult struct {
	Transcript              *models.JsonNullString
	Translation             *models.JsonNullString
	ShortSummary            *models.JsonNullString
	BulletedSummary         *models.JsonNullString
	VisualDescription       *models.JsonNullString
	MaterialType            *models.JsonNullString
	ConsolidatedCategories  []string
	VideoMentionedLocations []string // *** 欄位已正確定義 ***
	Keywords                []KeywordDisplay
	Bites                   []BiteDisplay
	ImportanceScore         *ImportanceScoreDisplay
	RelatedNews             []string
	ErrorMessage            *models.JsonNullString
	PromptVersion           string
}

// DashboardHandler 負責處理儀表板頁面的請求
type DashboardHandler struct {
	db       DBStore
	tpl      *template.Template
	basePath string
}

// NewDashboardHandler 建立一個 DashboardHandler 實例
func NewDashboardHandler(db DBStore, templateBasePath string) (*DashboardHandler, error) {
	if db == nil {
		return nil, fmt.Errorf("DBStore 不得為 nil")
	}
	tplPath := filepath.Join(templateBasePath, "dashboard.html")
	tpl, err := template.ParseFiles(tplPath)
	if err != nil {
		return nil, fmt.Errorf("無法解析儀表板範本 '%s': %w", tplPath, err)
	}
	return &DashboardHandler{db: db, tpl: tpl, basePath: templateBasePath}, nil
}

// getFlagForLocationGo (保持不變)
func getFlagForLocationGo(locationString string) string {
	if locationString == "" {
		return ""
	}
	locationLower := strings.ToLower(locationString)
	if strings.Contains(locationLower, "美國") || strings.Contains(locationLower, "u.s.") || strings.Contains(locationLower, "usa") || strings.Contains(locationLower, "華盛頓") {
		return "🇺🇸"
	}
	if strings.Contains(locationLower, "日本") || strings.Contains(locationLower, "japan") || strings.Contains(locationLower, "東京") {
		return "🇯🇵"
	}
	if strings.Contains(locationLower, "中國") || strings.Contains(locationLower, "china") || strings.Contains(locationLower, "北京") || strings.Contains(locationLower, "上海") || strings.Contains(locationLower, "山東") {
		return "🇨🇳"
	}
	if strings.Contains(locationLower, "台灣") || strings.Contains(locationLower, "taiwan") || strings.Contains(locationLower, "臺北") || strings.Contains(locationLower, "台北") {
		return "🇹🇼"
	}
	if strings.Contains(locationLower, "南非") || strings.Contains(locationLower, "south africa") || strings.Contains(locationLower, "約翰尼斯堡") {
		return "🇿🇦"
	}
	if strings.Contains(locationLower, "法國") || strings.Contains(locationLower, "france") || strings.Contains(locationLower, "巴黎") {
		return "🇫🇷"
	}
	if strings.Contains(locationLower, "英國") || strings.Contains(locationLower, "u.k.") || strings.Contains(locationLower, "britain") {
		return "🇬🇧"
	}
	if strings.Contains(locationLower, "以色列") || strings.Contains(locationLower, "israel") {
		return "🇮🇱"
	}
	if strings.Contains(locationLower, "加薩") || strings.Contains(locationLower, "gaza") {
		return "🇵🇸"
	}
	return "🏳️"
}

// ServeHTTP 實現 http.Handler 介面
func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("資訊：收到 %s %s 請求\n", r.Method, r.URL.Path)
	videos, analysisResults, err := h.db.GetAllVideosWithAnalysis(20, 0)
	if err != nil {
		log.Printf("錯誤：從資料庫獲取影片數據失敗: %v", err)
		http.Error(w, "無法載入儀表板數據", http.StatusInternalServerError)
		return
	}

	var displayData []VideoDisplayData
	analysisResultMap := make(map[int64]models.AnalysisResult)
	for _, ar := range analysisResults {
		analysisResultMap[ar.VideoID] = ar
	}

	for _, v := range videos {
		displayItem := VideoDisplayData{
			VideoID:         v.ID,
			SourceName:      v.SourceName,
			SourceID:        v.SourceID,
			NASPath:         v.NASPath,
			Title:           v.Title.String,
			AnalysisStatus:  v.AnalysisStatus,
			AnalysisResult:  nil,
			PublishedAt:     v.PublishedAt,
			DurationSecs:    v.DurationSecs,
			ShotlistContent: v.ShotlistContent,
			ViewLink:        v.ViewLink,
			PrimaryLocation: v.Location.String,
			FlagEmoji:       getFlagForLocationGo(v.Location.String),
			VideoURL:        fmt.Sprintf("/media/%s", v.NASPath),
		}
		if v.DurationSecs.Valid {
			displayItem.FormattedDurationMinutes = v.DurationSecs.Int64 / 60
			displayItem.FormattedDurationSeconds = v.DurationSecs.Int64 % 60
		}
		if len(v.Subjects) > 0 && string(v.Subjects) != "null" {
			var txtSubjects []string
			if errJ := json.Unmarshal(v.Subjects, &txtSubjects); errJ == nil {
				displayItem.PrimarySubjects = txtSubjects
			} else {
				log.Printf("警告：無法將 Video.Subjects JSON ('%s') 解析為 []string: %v", string(v.Subjects), errJ)
			}
		}

		if ar, ok := analysisResultMap[v.ID]; ok {
			displayableAR := &DisplayableAnalysisResult{
				PromptVersion:     ar.PromptVersion,
				Transcript:        ar.Transcript,
				Translation:       ar.Translation,
				ShortSummary:      ar.ShortSummary,
				BulletedSummary:   ar.BulletedSummary,
				VisualDescription: ar.VisualDescription,
				MaterialType:      ar.MaterialType,
				ErrorMessage:      ar.ErrorMessage,
			}

			var consolidatedCategoriesMap = make(map[string]bool)
			var consolidatedCategories []string
			for _, subj := range displayItem.PrimarySubjects {
				trimmedSubj := strings.TrimSpace(subj)
				if trimmedSubj != "" && !consolidatedCategoriesMap[trimmedSubj] {
					consolidatedCategoriesMap[trimmedSubj] = true
					consolidatedCategories = append(consolidatedCategories, trimmedSubj)
				}
			}
			if len(ar.Topics) > 0 && string(ar.Topics) != "null" {
				var geminiTopics []string
				if errJ := json.Unmarshal(ar.Topics, &geminiTopics); errJ == nil {
					for _, topic := range geminiTopics {
						trimmedTopic := strings.TrimSpace(topic)
						if trimmedTopic != "" && !consolidatedCategoriesMap[trimmedTopic] {
							consolidatedCategoriesMap[trimmedTopic] = true
							consolidatedCategories = append(consolidatedCategories, trimmedTopic)
						}
					}
				} else {
					log.Printf("警告：無法將 AnalysisResult.Topics JSON ('%s') 解析為 []string: %v", string(ar.Topics), errJ)
				}
			}
			sort.Strings(consolidatedCategories)
			displayableAR.ConsolidatedCategories = consolidatedCategories

			var geminiMentionedLocations []string
			if len(ar.MentionedLocations) > 0 && string(ar.MentionedLocations) != "null" {
				if errJ := json.Unmarshal(ar.MentionedLocations, &geminiMentionedLocations); errJ != nil {
					log.Printf("警告：無法將 MentionedLocations JSON ('%s') 解析為 []string: %v", string(ar.MentionedLocations), errJ)
				}
			}
			var videoOnlyMentionedLocations []string
			primaryLocLower := strings.ToLower(displayItem.PrimaryLocation)
			for _, loc := range geminiMentionedLocations {
				trimmedLoc := strings.TrimSpace(loc)
				if trimmedLoc != "" && strings.ToLower(trimmedLoc) != primaryLocLower {
					found := false
					for _, existingLoc := range videoOnlyMentionedLocations {
						if existingLoc == trimmedLoc {
							found = true
							break
						}
					}
					if !found {
						videoOnlyMentionedLocations = append(videoOnlyMentionedLocations, trimmedLoc)
					}
				}
			}
			displayableAR.VideoMentionedLocations = videoOnlyMentionedLocations // *** 賦值到正確的欄位 ***

			if len(ar.Keywords) > 0 && string(ar.Keywords) != "null" {
				var keywordsSlice []KeywordDisplay
				if errJ := json.Unmarshal(ar.Keywords, &keywordsSlice); errJ == nil {
					displayableAR.Keywords = keywordsSlice
				} else {
					log.Printf("警告：無法將 Keywords JSON ('%s') 解析為 []KeywordDisplay: %v", string(ar.Keywords), errJ)
				}
			}
			if len(ar.Bites) > 0 && string(ar.Bites) != "null" {
				var bitesSlice []BiteDisplay
				if errJ := json.Unmarshal(ar.Bites, &bitesSlice); errJ == nil {
					displayableAR.Bites = bitesSlice
				} else {
					log.Printf("警告：無法將 Bites JSON ('%s') 解析為 []BiteDisplay: %v", string(ar.Bites), errJ)
				}
			}
			if len(ar.ImportanceScore) > 0 && string(ar.ImportanceScore) != "null" {
				var scoreObj ImportanceScoreDisplay
				if errJ := json.Unmarshal(ar.ImportanceScore, &scoreObj); errJ == nil {
					displayableAR.ImportanceScore = &scoreObj
				} else {
					log.Printf("警告：無法將 ImportanceScore JSON ('%s') 解析為 ImportanceScoreDisplay: %v", string(ar.ImportanceScore), errJ)
				}
			}
			if len(ar.RelatedNews) > 0 && string(ar.RelatedNews) != "null" {
				var newsSlice []string
				if errJ := json.Unmarshal(ar.RelatedNews, &newsSlice); errJ == nil {
					displayableAR.RelatedNews = newsSlice
				} else {
					log.Printf("警告：無法將 RelatedNews JSON ('%s') 解析為 []string: %v", string(ar.RelatedNews), errJ)
				}
			}
			displayItem.AnalysisResult = displayableAR
		}
		displayData = append(displayData, displayItem)
	}

	pageData := DashboardPageData{Videos: displayData}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tpl.Execute(w, pageData); err != nil {
		log.Printf("錯誤：執行儀表板範本失敗: %v", err)
	}
}

// prettyPrintJSON (保持不變)
func prettyPrintJSON(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var obj interface{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		pretty, err := json.MarshalIndent(obj, "", "  ")
		if err == nil {
			return string(pretty)
		}
	}
	return string(raw)
}

// TriggerTextAnalysisHandler 和 TriggerVideoAnalysisHandler 相關程式碼 (已在各自檔案中，此處不重複)
// type TextAnalysisPipelineRunner interface { ExecuteTextAnalysisPipeline() error }
// type TriggerTextAnalysisHandler struct { /* ... */ }
// func NewTriggerTextAnalysisHandler(as TextAnalysisPipelineRunner) *TriggerTextAnalysisHandler { /* ... */ }
// func (h *TriggerTextAnalysisHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { /* ... */ }

// type VideoContentPipelineRunner interface { ExecuteVideoContentPipeline() error }
// type TriggerVideoAnalysisHandler struct { /* ... */ }
// func NewTriggerVideoAnalysisHandler(as VideoContentPipelineRunner) *TriggerVideoAnalysisHandler { /* ... */ }
// func (h *TriggerVideoAnalysisHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { /* ... */ }
