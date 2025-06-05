package main

import (
	"AiHackathon-admin/internal/config"
	"AiHackathon-admin/internal/models"
	"AiHackathon-admin/internal/storage/mysql"
	"AiHackathon-admin/internal/web/handlers"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	// 載入配置
	cfg, err := config.Load("configs", "config")
	if err != nil {
		log.Fatalf("無法載入配置: %v", err)
	}

	// 初始化資料庫連接
	db, err := mysql.NewMySQLStore(cfg.Database)
	if err != nil {
		log.Fatalf("無法連接到資料庫: %v", err)
	}
	defer db.Close()

	// 獲取所有影片和分析結果
	videos, analysisResults, err := db.GetAllVideosWithAnalysis(1000, 0, "", "importance", "desc")
	if err != nil {
		log.Fatalf("無法獲取影片數據: %v", err)
	}

	// 轉換為顯示格式
	displayData := convertToDisplayData(videos, analysisResults)

	// 根據重要性評分排序
	sort.Slice(displayData, func(i, j int) bool {
		// 如果兩個影片都有重要性評分，比較評分
		if displayData[i].AnalysisResult != nil && displayData[i].AnalysisResult.ImportanceScore != nil &&
			displayData[j].AnalysisResult != nil && displayData[j].AnalysisResult.ImportanceScore != nil {
			ratingI := getRatingWeight(displayData[i].AnalysisResult.ImportanceScore.OverallRating)
			ratingJ := getRatingWeight(displayData[j].AnalysisResult.ImportanceScore.OverallRating)
			return ratingI > ratingJ // 降冪排序
		}
		// 如果只有一個有評分，有評分的排在前面
		if displayData[i].AnalysisResult != nil && displayData[i].AnalysisResult.ImportanceScore != nil {
			return true
		}
		if displayData[j].AnalysisResult != nil && displayData[j].AnalysisResult.ImportanceScore != nil {
			return false
		}
		// 如果都沒有評分，按 ID 排序
		return displayData[i].VideoID > displayData[j].VideoID
	})

	// 讀取模板
	tplPath := filepath.Join("internal", "web", "templates", "dashboard.html")
	tpl, err := template.ParseFiles(tplPath)
	if err != nil {
		log.Fatalf("無法解析模板: %v", err)
	}

	// 創建輸出目錄
	outputDir := "static"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("無法創建輸出目錄: %v", err)
	}

	// 生成靜態檔案
	outputFile := filepath.Join(outputDir, "index.html")
	file, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("無法創建輸出檔案: %v", err)
	}
	defer file.Close()

	// 準備頁面數據
	pageData := handlers.DashboardPageData{
		Videos:     displayData,
		SearchTerm: "",
		SortBy:     "importance",
		SortOrder:  "desc",
	}

	// 執行模板
	if err := tpl.Execute(file, pageData); err != nil {
		log.Fatalf("無法執行模板: %v", err)
	}

	log.Printf("靜態檔案已生成: %s", outputFile)
}

func convertToDisplayData(videos []models.Video, analysisResults []models.AnalysisResult) []handlers.VideoDisplayData {
	// 建立分析結果的映射
	analysisMap := make(map[int64]*models.AnalysisResult)
	for i, result := range analysisResults {
		if result.VideoID > 0 {
			analysisMap[result.VideoID] = &analysisResults[i]
		}
	}

	var displayData []handlers.VideoDisplayData
	for _, video := range videos {
		// 轉換為顯示格式
		display := handlers.VideoDisplayData{
			VideoID:          video.ID,
			SourceName:       video.SourceName,
			SourceID:         video.SourceID,
			CombinedSourceID: fmt.Sprintf("%s(%s)", strings.ToUpper(video.SourceName), video.SourceID),
			NASPath:          video.NASPath,
			Title:            video.Title.String,
			AnalysisStatus:   video.AnalysisStatus,
			PublishedAt:      video.PublishedAt,
			FetchedAt:        video.FetchedAt,
			DurationSecs:     video.DurationSecs,
			ShotlistContent:  video.ShotlistContent,
			ViewLink:         video.ViewLink,
			PromptVersion:    video.PromptVersion,
		}

		// 計算時長
		if video.DurationSecs.Valid {
			display.FormattedDurationMinutes = video.DurationSecs.Int64 / 60
			display.FormattedDurationSeconds = video.DurationSecs.Int64 % 60
		}

		// 處理分析結果
		if result, ok := analysisMap[video.ID]; ok {
			display.AnalysisResult = convertAnalysisResult(result)
		}

		// 處理位置和主題
		if display.AnalysisResult != nil && len(display.AnalysisResult.VideoMentionedLocations) > 0 {
			display.PrimaryLocation = display.AnalysisResult.VideoMentionedLocations[0]
			display.FlagEmoji = getFlagForLocation(display.PrimaryLocation)
		}
		if display.AnalysisResult != nil && len(display.AnalysisResult.ConsolidatedCategories) > 0 {
			display.PrimarySubjects = display.AnalysisResult.ConsolidatedCategories
		}

		// 設定影片 URL
		display.VideoURL = fmt.Sprintf("%s/%s.mp4", video.SourceID, video.SourceID)

		displayData = append(displayData, display)
	}

	return displayData
}

func convertAnalysisResult(result *models.AnalysisResult) *handlers.DisplayableAnalysisResult {
	if result == nil {
		return nil
	}

	display := &handlers.DisplayableAnalysisResult{
		Transcript:        result.Transcript,
		Translation:       result.Translation,
		ShortSummary:      result.ShortSummary,
		BulletedSummary:   result.BulletedSummary,
		VisualDescription: result.VisualDescription,
		MaterialType:      result.MaterialType,
		ErrorMessage:      result.ErrorMessage,
		PromptVersion:     result.PromptVersion,
		AnalysisCreatedAt: result.CreatedAt,
	}

	// 解析 JSON 欄位
	if len(result.Topics) > 0 {
		var topics []string
		if err := json.Unmarshal(result.Topics, &topics); err == nil {
			display.ConsolidatedCategories = topics
		}
	}

	if len(result.MentionedLocations) > 0 {
		var locations []string
		if err := json.Unmarshal(result.MentionedLocations, &locations); err == nil {
			display.VideoMentionedLocations = locations
		}
	}

	if len(result.Keywords) > 0 {
		var keywords []handlers.KeywordDisplay
		if err := json.Unmarshal(result.Keywords, &keywords); err == nil {
			display.Keywords = keywords
		}
	}

	if len(result.Bites) > 0 {
		var bites []handlers.BiteDisplay
		if err := json.Unmarshal(result.Bites, &bites); err == nil {
			display.Bites = bites
		}
	}

	if len(result.ImportanceScore) > 0 {
		var importance handlers.ImportanceScoreDisplay
		if err := json.Unmarshal(result.ImportanceScore, &importance); err == nil {
			display.ImportanceScore = &importance
		}
	}

	if len(result.RelatedNews) > 0 {
		var relatedNews []string
		if err := json.Unmarshal(result.RelatedNews, &relatedNews); err == nil {
			display.RelatedNews = relatedNews
		}
	}

	return display
}

func getFlagForLocation(locationString string) string {
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
