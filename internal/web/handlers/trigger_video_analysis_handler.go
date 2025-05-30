package handlers

import (
	// "AiHackathon-admin/internal/services" // 註解掉，因為介面已在此檔案定義
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
		w.WriteHeader(http.StatusConflict) // 409 Conflict
		json.NewEncoder(w).Encode(map[string]string{"error": "影片內容分析任務已在進行中，請稍候。"})
		return
	}
	h.isProcessing = true
	h.mu.Unlock()

	log.Println("資訊：[TriggerVideoAnalysisHandler] 收到手動觸發影片內容分析請求，準備啟動 goroutine。")

	go func() {
		defer func() {
			h.mu.Lock()
			h.isProcessing = false
			h.mu.Unlock()
			log.Println("資訊：[TriggerVideoAnalysisHandler] 手動觸發的影片內容分析任務 goroutine 已結束。")
		}()

		log.Println("資訊：[TriggerVideoAnalysisHandler] 開始執行手動觸發的影片內容分析任務...")
		err := h.analyzeService.ExecuteVideoContentPipeline()
		if err != nil {
			log.Printf("錯誤：[TriggerVideoAnalysisHandler] 手動觸發的影片內容分析任務執行失敗: %v", err)
		} else {
			log.Println("資訊：[TriggerVideoAnalysisHandler] 手動觸發的影片內容分析任務執行成功。")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "影片內容分析已觸發，正在背景執行。"})
}
