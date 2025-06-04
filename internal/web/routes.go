package web

import (
	"AiHackathon-admin/internal/config"
	"AiHackathon-admin/internal/services"
	"AiHackathon-admin/internal/web/handlers"
	"log"
	"net/http"
	// 新增：用於 http.StripPrefix
)

// AnalysisServiceRunner (保持不變)
type AnalysisServiceRunner interface {
	handlers.TextAnalysisPipelineRunner
	handlers.VideoContentPipelineRunner
}

// SetupRouter 更新：接收 config.NASConfig
func SetupRouter(appConfig *config.Config, db handlers.DBStore, analyzeService *services.AnalyzeService) http.Handler {
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
		// 如果不是 /dashboard 也不是 /media/，則 404
		// 注意：/media/ 的處理在下面，如果這裡直接 NotFound，/media/ 可能無法匹配
		// 因此，根路徑的處理可以更精確，或依賴於 ServeMux 的匹配順序
	})

	// 手動觸發分析的路由 (保持不變)
	if analyzeService == nil {
		log.Panicln("SetupRouter：AnalyzeService 不得為空")
	}
	triggerTextAnalysisHandler := handlers.NewTriggerTextAnalysisHandler(analyzeService)
	mux.Handle("/manual-text-analyze", triggerTextAnalysisHandler)
	triggerVideoAnalysisHandler := handlers.NewTriggerVideoAnalysisHandler(analyzeService)
	mux.Handle("/manual-video-analyze", triggerVideoAnalysisHandler)

	// 匯出處理器
	exportHandler := handlers.NewExportHandler(db)
	mux.Handle("/export", exportHandler)

	// --- 新增：影片串流服務路由 ---
	videoHandler, err := handlers.NewVideoHandler(appConfig.NAS) // 使用 appConfig.NAS
	if err != nil {
		log.Fatalf("錯誤：無法建立 Video Handler: %v", err)
	}
	// http.StripPrefix 會移除 "/media/" 前綴，然後將剩餘路徑傳遞給 videoHandler
	// videoHandler 的 ServeHTTP 內部需要再次處理這個相對路徑以構建完整檔案路徑
	mux.Handle("/media/", http.StripPrefix("/media/", videoHandler))
	// --- 結束新增 ---

	// 將根路徑的 NotFound 處理移到最後，確保其他 Handle 被優先匹配
	mux.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		// 再次檢查，如果真的是根路徑 "/"，則重定向（避免之前的 HandleFunc "/" 被覆蓋）
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
		log.Printf("警告：未匹配的路由: %s", r.URL.Path)
		http.NotFound(w, r)
	})

	log.Println("資訊：HTTP 路由設定完成。")
	return mux
}
