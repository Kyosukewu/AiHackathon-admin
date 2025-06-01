package handlers

import (
	// "AiHackathon-admin/internal/services" // 介面在此檔案定義，不需要直接 import services
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// VideoContentPipelineRunner 定義了影片內容分析流程執行者需要的方法
type VideoContentPipelineRunner interface {
	ExecuteVideoContentPipeline() error
}

// TriggerVideoAnalysisHandler 負責處理手動觸發影片內容分析的請求
type TriggerVideoAnalysisHandler struct {
	analyzeService VideoContentPipelineRunner // 依賴介面
	mu             sync.Mutex
	isProcessing   bool
}

// NewTriggerVideoAnalysisHandler 建立一個 TriggerVideoAnalysisHandler 實例
func NewTriggerVideoAnalysisHandler(as VideoContentPipelineRunner) *TriggerVideoAnalysisHandler {
	if as == nil {
		log.Panicln("TriggerVideoAnalysisHandler：VideoContentPipelineRunner 不得為空")
	}
	log.Println("DEBUG: [TriggerVideoAnalysisHandler] NewTriggerVideoAnalysisHandler called, analyzeService is NOT nil.") // 新增日誌
	return &TriggerVideoAnalysisHandler{
		analyzeService: as,
	}
}

// ServeHTTP 實現 http.Handler 介面
func (h *TriggerVideoAnalysisHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("資訊：[TriggerVideoAnalysisHandler] 收到請求: %s %s 來自 %s\n", r.Method, r.URL.Path, r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Printf("警告：[TriggerVideoAnalysisHandler] 收到非 POST 請求 (%s)，已拒絕。\n", r.Method)
		http.Error(w, "僅支援 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	h.mu.Lock()
	if h.isProcessing {
		h.mu.Unlock()
		log.Println("警告：[TriggerVideoAnalysisHandler] 影片內容分析已在進行中，請稍後再試。")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "影片內容分析任務已在進行中，請稍候。"})
		return
	}
	h.isProcessing = true
	h.mu.Unlock()

	log.Println("資訊：[TriggerVideoAnalysisHandler] 收到手動觸發影片內容分析請求，準備啟動 goroutine。")
	// *** 新增日誌：確認 analyzeService 是否為 nil ***
	if h.analyzeService == nil {
		log.Println("錯誤：[TriggerVideoAnalysisHandler] h.analyzeService 是 nil，無法啟動分析！")
		// 可以在這裡回傳一個內部錯誤給前端
		h.mu.Lock()
		h.isProcessing = false // 重設狀態
		h.mu.Unlock()
		http.Error(w, "內部伺服器錯誤 (service not initialized)", http.StatusInternalServerError)
		return
	}
	log.Println("DEBUG: [TriggerVideoAnalysisHandler] h.analyzeService is NOT nil, about to start goroutine.")
	// *** 結束新增日誌 ***

	go func() {
		defer func() {
			h.mu.Lock()
			h.isProcessing = false
			h.mu.Unlock()
			log.Println("資訊：[TriggerVideoAnalysisHandler] 手動觸發的影片內容分析任務 goroutine 已結束。")
		}()

		log.Println("資訊：[TriggerVideoAnalysisHandler] goroutine 已啟動，準備呼叫 ExecuteVideoContentPipeline...")
		err := h.analyzeService.ExecuteVideoContentPipeline() // 呼叫介面方法
		if err != nil {
			log.Printf("錯誤：[TriggerVideoAnalysisHandler] 手動觸發的影片內容分析任務執行失敗: %v", err)
		} else {
			log.Println("資訊：[TriggerVideoAnalysisHandler] 手動觸發的影片內容分析任務執行回傳成功。")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "影片內容分析已觸發，正在背景執行。"})
}
