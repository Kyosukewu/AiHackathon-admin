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
	"time"
)

// DBStore 介面更新：GetAllVideosWithAnalysis 現在接收篩選和排序參數
type DBStore interface {
	GetAllVideosWithAnalysis(limit int, offset int, searchTerm string, sortBy string, sortOrder string) ([]models.Video, []models.AnalysisResult, error)
	Close() error
	FindOrCreateVideo(video *models.Video) (int64, error)
	SaveAnalysisResult(result *models.AnalysisResult) error
	UpdateVideoAnalysisStatus(videoID int64, status models.AnalysisStatus, analyzedAt sql.NullTime, errorMessage sql.NullString) error
	GetPendingVideos(limit int) ([]models.Video, error)
	GetVideoByID(videoID int64) (*models.Video, error)
	GetVideosPendingContentAnalysis(status models.AnalysisStatus, limit int) ([]models.Video, error)
}

// DashboardPageData 更新：加入篩選和排序的當前值，以便在範本中設定表單預設值
type DashboardPageData struct {
	Videos     []VideoDisplayData
	SearchTerm string
	SortBy     string
	SortOrder  string
	Paging     PagingData // 可選：用於將來實現分頁
}

// PagingData (可選，為將來分頁做準備)
type PagingData struct {
	CurrentPage int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
}

// VideoDisplayData 更新：加入 CombinedSourceID
type VideoDisplayData struct {
	VideoID                  int64
	SourceName               string // 保留原始 SourceName
	SourceID                 string // 保留原始 SourceID
	CombinedSourceID         string // *** 新增：用於顯示 SourceName(大寫) + SourceID ***
	NASPath                  string
	Title                    string
	AnalysisStatus           models.AnalysisStatus
	AnalysisResult           *DisplayableAnalysisResult
	PublishedAt              sql.NullTime
	FetchedAt                time.Time
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

// KeywordDisplay, BiteDisplay, ImportanceScoreDisplay, DisplayableAnalysisResult (保持不變)
type KeywordDisplay struct {
	Keyword  string `json:"keyword"`
	Category string `json:"category"`
}
type BiteDisplay struct {
	Speaker string `json:"speaker"`
	Quote   string `json:"quote"`
}
type ImportanceScoreDisplay struct {
	OverallRating     string   `json:"overall_rating"`
	KeyFactors        []string `json:"key_factors"`
	AssessmentDetails string   `json:"assessment_details"`
}
type DisplayableAnalysisResult struct {
	Transcript              *models.JsonNullString
	Translation             *models.JsonNullString
	ShortSummary            *models.JsonNullString
	BulletedSummary         *models.JsonNullString
	VisualDescription       *models.JsonNullString
	MaterialType            *models.JsonNullString
	ConsolidatedCategories  []string
	VideoMentionedLocations []string
	Keywords                []KeywordDisplay
	Bites                   []BiteDisplay
	ImportanceScore         *ImportanceScoreDisplay
	RelatedNews             []string
	ErrorMessage            *models.JsonNullString
	PromptVersion           string
	AnalysisCreatedAt       time.Time
}

// DashboardHandler (保持不變)
type DashboardHandler struct {
	db       DBStore
	tpl      *template.Template
	basePath string
}

// NewDashboardHandler (保持不變)
func NewDashboardHandler(db DBStore, templateBasePath string) (*DashboardHandler, error) {
	if db == nil {
		return nil, fmt.Errorf("DBStore不得為nil")
	}
	tplPath := filepath.Join(templateBasePath, "dashboard.html")
	tpl, err := template.ParseFiles(tplPath)
	if err != nil {
		return nil, fmt.Errorf("無法解析儀表板範本 '%s': %w", tplPath, err)
	}
	return &DashboardHandler{db: db, tpl: tpl, basePath: templateBasePath}, nil
}
func getFlagForLocationGo(locationString string) string { /* ... */
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
func getRatingWeight(rating string) int {
	upperRating := strings.ToUpper(rating)
	switch upperRating {
	case "S":
		return 5
	case "A":
		return 4
	case "B":
		return 3
	case "C":
		return 2
	case "N":
		return 1
	default:
		return 0
	}
}

// ServeHTTP 方法更新：讀取查詢參數，傳遞給資料庫層，並調整排序邏輯
func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("資訊：收到 %s %s 請求\n", r.Method, r.URL.Path)

	// 從 URL 查詢參數讀取篩選和排序條件
	searchTerm := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")

	// 設定預設排序（如果前端未提供）
	if sortBy == "" {
		sortBy = "importance" // 預設按評分排序
	}
	if sortOrder == "" {
		sortOrder = "desc" // 預設降冪
	}

	// TODO: 實現分頁邏輯 (limit, offset)
	limit := 20 // 暫時固定每頁數量
	offset := 0

	videos, analysisResults, err := h.db.GetAllVideosWithAnalysis(limit, offset, searchTerm, sortBy, sortOrder)
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
			VideoID:    v.ID,
			SourceName: v.SourceName, // 保留原始 SourceName
			SourceID:   v.SourceID,   // 保留原始 SourceID
			// *** 填充 CombinedSourceID ***
			CombinedSourceID: fmt.Sprintf("%s%s", strings.ToUpper(v.SourceName), v.SourceID),
			NASPath:          v.NASPath, Title: v.Title.String, AnalysisStatus: v.AnalysisStatus,
			AnalysisResult: nil, PublishedAt: v.PublishedAt, FetchedAt: v.FetchedAt,
			DurationSecs: v.DurationSecs, ShotlistContent: v.ShotlistContent, ViewLink: v.ViewLink,
			PrimaryLocation: v.Location.String, FlagEmoji: getFlagForLocationGo(v.Location.String),
			VideoURL: fmt.Sprintf("/media/%s", v.NASPath),
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
				log.Printf("警告：[DashboardHandler] 無法將 Video.Subjects (ID: %d) JSON ('%s') 解析為 []string: %v。", v.ID, string(v.Subjects), errJ)
			}
		}

		if ar, ok := analysisResultMap[v.ID]; ok {
			displayableAR := &DisplayableAnalysisResult{
				PromptVersion: ar.PromptVersion, Transcript: ar.Transcript, Translation: ar.Translation,
				ShortSummary: ar.ShortSummary, BulletedSummary: ar.BulletedSummary,
				VisualDescription: ar.VisualDescription, MaterialType: ar.MaterialType, ErrorMessage: ar.ErrorMessage,
				AnalysisCreatedAt: ar.CreatedAt,
			}
			var consolidatedCategoriesMap = make(map[string]bool)
			var consolidatedCategories []string
			for _, subj := range displayItem.PrimarySubjects {
				trimmedSubj := strings.TrimSpace(subj)
				if trimmedSubj != "" && !strings.HasPrefix(trimmedSubj, "原始數據(解析失敗):") && !consolidatedCategoriesMap[trimmedSubj] {
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
					log.Printf("警告：[DashboardHandler] 無法將 AnalysisResult.Topics (VideoID: %d) JSON ('%s') 解析為 []string: %v。", ar.VideoID, string(ar.Topics), errJ)
					consolidatedCategories = append(consolidatedCategories, fmt.Sprintf("原始Topics(解析失敗): %s", string(ar.Topics)))
				}
			}
			sort.Strings(consolidatedCategories)
			displayableAR.ConsolidatedCategories = consolidatedCategories

			parseAndSet := func(raw json.RawMessage, target interface{}, fieldName string) {
				if len(raw) > 0 && string(raw) != "null" {
					if errJ := json.Unmarshal(raw, target); errJ != nil {
						log.Printf("警告：[DashboardHandler] 無法將 %s (VideoID: %d) JSON ('%s') 解析: %v", fieldName, ar.VideoID, string(raw), errJ)
					}
				}
			}
			parseAndSet(ar.MentionedLocations, &displayableAR.VideoMentionedLocations, "MentionedLocations")
			parseAndSet(ar.Keywords, &displayableAR.Keywords, "Keywords")
			parseAndSet(ar.Bites, &displayableAR.Bites, "Bites")
			parseAndSet(ar.ImportanceScore, &displayableAR.ImportanceScore, "ImportanceScore")
			parseAndSet(ar.RelatedNews, &displayableAR.RelatedNews, "RelatedNews")

			displayItem.AnalysisResult = displayableAR
		}
		displayData = append(displayData, displayItem)
	}

	// --- 修改：只有在 sortBy 是 "importance" 時才在 Go 中排序 ---
	// 其他排序（published_at, source_id）將依賴資料庫層的 ORDER BY
	if sortBy == "importance" {
		sort.Slice(displayData, func(i, j int) bool {
			var ratingI, ratingJ int
			var timeI, timeJ time.Time // 用於評分相同時的次要排序

			if displayData[i].AnalysisResult != nil && displayData[i].AnalysisResult.ImportanceScore != nil {
				ratingI = getRatingWeight(displayData[i].AnalysisResult.ImportanceScore.OverallRating)
			} else {
				ratingI = -1
			} // 未評分或無分析結果的排在後面

			if displayData[j].AnalysisResult != nil && displayData[j].AnalysisResult.ImportanceScore != nil {
				ratingJ = getRatingWeight(displayData[j].AnalysisResult.ImportanceScore.OverallRating)
			} else {
				ratingJ = -1
			}

			// 根據 sortOrder 決定升冪還是降冪
			if ratingI != ratingJ {
				if sortOrder == "asc" {
					return ratingI < ratingJ
				}
				return ratingI > ratingJ // 預設降冪
			}

			// 評分相同，使用發布時間作為次要排序 (較新的在前)
			if displayData[i].PublishedAt.Valid {
				timeI = displayData[i].PublishedAt.Time
			} else if !displayData[i].FetchedAt.IsZero() {
				timeI = displayData[i].FetchedAt
			} else if displayData[i].AnalysisResult != nil && !displayData[i].AnalysisResult.AnalysisCreatedAt.IsZero() {
				timeI = displayData[i].AnalysisResult.AnalysisCreatedAt
			} else {
				timeI = time.Time{}
			}

			if displayData[j].PublishedAt.Valid {
				timeJ = displayData[j].PublishedAt.Time
			} else if !displayData[j].FetchedAt.IsZero() {
				timeJ = displayData[j].FetchedAt
			} else if displayData[j].AnalysisResult != nil && !displayData[j].AnalysisResult.AnalysisCreatedAt.IsZero() {
				timeJ = displayData[j].AnalysisResult.AnalysisCreatedAt
			} else {
				timeJ = time.Time{}
			}

			if sortOrder == "asc" {
				return timeI.Before(timeJ)
			}
			return timeI.After(timeJ) // 預設降冪 (新的在前)
		})
	}
	// --- 結束排序修改 ---

	pageData := DashboardPageData{
		Videos:     displayData,
		SearchTerm: searchTerm,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tpl.Execute(w, pageData); err != nil {
		log.Printf("錯誤：執行儀表板範本失敗: %v", err)
	}
}
