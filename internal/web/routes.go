package web

import (
	"AiHackathon-admin/internal/config" // 引入 services
	"AiHackathon-admin/internal/web/handlers"
	"log"
	"net/http"
)

// SetupRouter 設定應用程式的所有 HTTP 路由
// 更新簽名以接收 AnalyzeRunner (AnalyzeService 的實例)
func SetupRouter(cfg *config.Config, db handlers.DBStore, analyzeService handlers.AnalyzeRunner) http.Handler {
	mux := http.NewServeMux()
	templateBasePath := "internal/web/templates"

	// Dashboard Handler (保持不變)
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

	// --- 新增：手動觸發分析的路由和 Handler ---
	if analyzeService == nil {
		log.Panicln("SetupRouter：AnalyzeRunner 不得為空，無法設定 /manual-analyze 路由")
	}
	triggerAnalysisHandler := handlers.NewTriggerAnalysisHandler(analyzeService)
	mux.Handle("/manual-analyze", triggerAnalysisHandler) // POST /manual-analyze
	// --- 結束新增 ---

	log.Println("資訊：HTTP 路由設定完成。")
	return mux
}
