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
)

// DBStore 介面 (保持不變)
type DBStore interface {
	GetAllVideosWithAnalysis(limit int, offset int) ([]models.Video, []models.AnalysisResult, error)
	Close() error
	FindOrCreateVideo(videoInfo models.VideoFileInfo) (int64, error)
	SaveAnalysisResult(result *models.AnalysisResult) error
	UpdateVideoAnalysisStatus(videoID int64, status models.AnalysisStatus, analyzedAt sql.NullTime, errorMessage sql.NullString) error
	GetPendingVideos(limit int) ([]models.Video, error)
	GetVideoByID(videoID int64) (*models.Video, error)
}

// DashboardPageData (保持不變)
type DashboardPageData struct{ Videos []VideoDisplayData }

// VideoDisplayData (保持不變)
type VideoDisplayData struct {
	VideoID        int64
	SourceName     string
	SourceID       string
	NASPath        string
	Title          string
	AnalysisStatus models.AnalysisStatus
	AnalysisResult *DisplayableAnalysisResult // 指向 DisplayableAnalysisResult
}

// KeywordDisplay (保持不變)
type KeywordDisplay struct {
	Keyword  string `json:"keyword"`
	Category string `json:"category"`
}

// DisplayableAnalysisResult 更新：欄位類型改為 *models.JsonNullString
type DisplayableAnalysisResult struct {
	Transcript        *models.JsonNullString // 改為指標
	Translation       *models.JsonNullString // 改為指標
	Summary           *models.JsonNullString // 改為指標
	VisualDescription *models.JsonNullString // 改為指標
	Topics            []string
	Keywords          []KeywordDisplay
	ErrorMessage      *models.JsonNullString // 改為指標
	PromptVersion     string
}

// DashboardHandler (保持不變)
type DashboardHandler struct {
	db       DBStore
	tpl      *template.Template
	basePath string
}

// NewDashboardHandler (保持不變)
func NewDashboardHandler(db DBStore, templateBasePath string) (*DashboardHandler, error) {
	tplPath := filepath.Join(templateBasePath, "dashboard.html")
	tpl, err := template.ParseFiles(tplPath)
	if err != nil {
		return nil, fmt.Errorf("無法解析儀表板範本 '%s': %w", tplPath, err)
	}
	return &DashboardHandler{db: db, tpl: tpl, basePath: templateBasePath}, nil
}

// ServeHTTP 方法更新 (處理 *JsonNullString)
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
			VideoID: v.ID, SourceName: v.SourceName, SourceID: v.SourceID, NASPath: v.NASPath,
			Title: v.Title.String, AnalysisStatus: v.AnalysisStatus, AnalysisResult: nil,
		}
		if ar, ok := analysisResultMap[v.ID]; ok {
			displayableAR := &DisplayableAnalysisResult{ // 初始化時，指標欄位預設為 nil
				PromptVersion: ar.PromptVersion,
			}
			// 如果 ar 中的指標不是 nil，則將其賦值
			if ar.Transcript != nil {
				displayableAR.Transcript = ar.Transcript
			}
			if ar.Translation != nil {
				displayableAR.Translation = ar.Translation
			}
			if ar.Summary != nil {
				displayableAR.Summary = ar.Summary
			}
			if ar.VisualDescription != nil {
				displayableAR.VisualDescription = ar.VisualDescription
			}
			if ar.ErrorMessage != nil {
				displayableAR.ErrorMessage = ar.ErrorMessage
			}

			if len(ar.Topics) > 0 && string(ar.Topics) != "null" {
				var topicsSlice []string
				if err := json.Unmarshal(ar.Topics, &topicsSlice); err == nil {
					displayableAR.Topics = topicsSlice
				} else {
					log.Printf("警告：無法將 Topics JSON ('%s') 解析為 []string: %v", string(ar.Topics), err)
				}
			}
			if len(ar.Keywords) > 0 && string(ar.Keywords) != "null" {
				var keywordsSlice []KeywordDisplay
				if err := json.Unmarshal(ar.Keywords, &keywordsSlice); err == nil {
					displayableAR.Keywords = keywordsSlice
				} else {
					log.Printf("警告：無法將 Keywords JSON ('%s') 解析為 []KeywordDisplay: %v", string(ar.Keywords), err)
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
		http.Error(w, "渲染儀表板頁面時發生錯誤", http.StatusInternalServerError)
	}
}
