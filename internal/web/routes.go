package web

import (
	"AiHackathon-admin/internal/config"
	"AiHackathon-admin/internal/services" // 確保引入 services
	"AiHackathon-admin/internal/web/handlers"
	"log"
	"net/http"
)

// AnalysisServiceRunner 介面現在作為一個標記，表示 analyzeService 應包含兩個 pipeline 方法
// handlers 內部會使用更具體的介面
// 或者，我們可以讓 SetupRouter 直接接收 *services.AnalyzeService
// 為了清晰，我們讓 SetupRouter 接收 *services.AnalyzeService
func SetupRouter(cfg *config.Config, db handlers.DBStore, analyzeService *services.AnalyzeService) http.Handler {
	mux := http.NewServeMux()
	templateBasePath := "internal/web/templates"

	// Dashboard Handler
	dashboardHandler, err := handlers.NewDashboardHandler(db, templateBasePath)
	if err != nil {
		log.Fatalf("錯誤：無法建立 Dashboard Handler: %v", err)
	}
	mux.Handle("/dashboard", dashboardHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// 防禦性檢查
	if analyzeService == nil {
		log.Panicln("SetupRouter：AnalyzeService 不得為空")
	}

	// 手動觸發文本元數據分析的路由和 Handler
	// analyzeService 實例同時實現了 TextAnalysisPipelineRunner 和 VideoContentPipelineRunner
	triggerTextAnalysisHandler := handlers.NewTriggerTextAnalysisHandler(analyzeService)
	mux.Handle("/manual-text-analyze", triggerTextAnalysisHandler)

	// 手動觸發影片內容分析的路由和 Handler
	triggerVideoAnalysisHandler := handlers.NewTriggerVideoAnalysisHandler(analyzeService)
	mux.Handle("/manual-video-analyze", triggerVideoAnalysisHandler)

	log.Println("資訊：HTTP 路由設定完成。")
	return mux
}
