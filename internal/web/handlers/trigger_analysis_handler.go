package handlers

import (
	// 引入 services 套件
	"encoding/json"
	"log"
	"net/http"
	"sync" // 用於確保一次只有一個手動分析在運行（可選）
)

// AnalyzeRunner 定義了 AnalyzeService 中 Run 方法的介面
// 這樣 TriggerAnalysisHandler 就不直接依賴 AnalyzeService 的具體實現
type AnalyzeRunner interface {
	Run() error
}

// TriggerAnalysisHandler 負責處理手動觸發影片分析的請求
type TriggerAnalysisHandler struct {
	analyzeService AnalyzeRunner // 使用介面
	mu             sync.Mutex    // 用於鎖定，避免同時觸發多次分析
	isAnalyzing    bool          // 標記是否正在分析
}

// NewTriggerAnalysisHandler 建立一個 TriggerAnalysisHandler 實例
func NewTriggerAnalysisHandler(as AnalyzeRunner) *TriggerAnalysisHandler {
	if as == nil {
		log.Panicln("TriggerAnalysisHandler：AnalyzeRunner 不得為空") // 使用 Panic 因為這是程式設定錯誤
	}
	return &TriggerAnalysisHandler{
		analyzeService: as,
	}
}

// ServeHTTP 實現 http.Handler 介面
func (h *TriggerAnalysisHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "僅支援 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	h.mu.Lock() // 加鎖以檢查和設定 isAnalyzing 狀態
	if h.isAnalyzing {
		h.mu.Unlock()
		log.Println("警告：手動分析已在進行中，請稍後再試。")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict) // 409 Conflict
		json.NewEncoder(w).Encode(map[string]string{"error": "分析任務已在進行中，請稍候。"})
		return
	}
	h.isAnalyzing = true
	h.mu.Unlock()

	log.Println("資訊：收到手動觸發影片分析請求。")

	// 在一個新的 goroutine 中執行分析，以避免阻塞 HTTP 回應
	go func() {
		defer func() { // 確保在 goroutine 結束時重設狀態
			h.mu.Lock()
			h.isAnalyzing = false
			h.mu.Unlock()
			log.Println("資訊：手動觸發的分析任務 goroutine 已結束。")
		}()

		log.Println("資訊：開始執行手動觸發的影片分析任務...")
		err := h.analyzeService.Run() // 呼叫 AnalyzeService 的 Run 方法
		if err != nil {
			log.Printf("錯誤：手動觸發的影片分析任務執行失敗: %v", err)
			// 這裡的錯誤不會直接回傳給前端的 POST 回應，因為 POST 已先回傳
			// 您可以考慮將任務狀態/結果儲存到資料庫，供 Dashboard 查詢
		} else {
			log.Println("資訊：手動觸發的影片分析任務執行成功。")
		}
	}()

	// 立即回傳成功訊息給前端
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "影片分析已觸發，正在背景執行。請稍後查看結果。"})
}
