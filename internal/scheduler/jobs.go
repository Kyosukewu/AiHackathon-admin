package scheduler

import (
	"AiHackathon-admin/internal/services"
	"log"
)

// FetchJob 是一個排程任務，用於執行影片擷取
type FetchJob struct {
	fetchService *services.FetchService
}

// NewFetchJob 建立一個 FetchJob
func NewFetchJob(fs *services.FetchService) *FetchJob {
	return &FetchJob{fetchService: fs}
}

// Run 實現 cron.Job 介面 (github.com/robfig/cron/v3)
func (j *FetchJob) Run() {
	log.Println("資訊：執行排程任務 - 影片擷取...")
	if err := j.fetchService.Run(); err != nil {
		log.Printf("錯誤：影片擷取排程任務執行失敗: %v", err)
	} else {
		log.Println("資訊：影片擷取排程任務執行完成。")
	}
}

// AnalyzeJob 是一個排程任務，用於執行影片分析
type AnalyzeJob struct {
	analyzeService *services.AnalyzeService
}

// NewAnalyzeJob 建立一個 AnalyzeJob
func NewAnalyzeJob(as *services.AnalyzeService) *AnalyzeJob {
	return &AnalyzeJob{analyzeService: as}
}

// Run 實現 cron.Job 介面
func (j *AnalyzeJob) Run() {
	log.Println("資訊：執行排程任務 - 影片分析...")
	if err := j.analyzeService.Run(); err != nil {
		log.Printf("錯誤：影片分析排程任務執行失敗: %v", err)
	} else {
		log.Println("資訊：影片分析排程任務執行完成。")
	}
}
