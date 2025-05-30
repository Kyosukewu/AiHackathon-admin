package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// AnalyzeRunner (保持不變)
type AnalyzeRunner interface {
	Run() error
}

// TriggerAnalysisHandler (保持不變)
type TriggerAnalysisHandler struct {
	analyzeService AnalyzeRunner
	mu             sync.Mutex
	isAnalyzing    bool
}

// NewTriggerAnalysisHandler (保持不變)
func NewTriggerAnalysisHandler(as AnalyzeRunner) *TriggerAnalysisHandler {
	if as == nil {
		log.Panicln("TriggerAnalysisHandler：AnalyzeRunner 不得為空")
	}
	return &TriggerAnalysisHandler{
		analyzeService: as,
	}
}

// ServeHTTP 實現 http.Handler 介面
func (h *TriggerAnalysisHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// *** 新增：在方法開始時就記錄日誌 ***
	log.Printf("資訊：[TriggerAnalysisHandler] 收到請求: %s %s 來自 %s\n", r.Method, r.URL.Path, r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Printf("警告：[TriggerAnalysisHandler] 收到非 POST 請求 (%s)，已拒絕。\n", r.Method)
		http.Error(w, "僅支援 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	h.mu.Lock()
	if h.isAnalyzing {
		h.mu.Unlock()
		log.Println("警告：[TriggerAnalysisHandler] 手動分析已在進行中，拒絕新的觸發。")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict) // 409 Conflict
		json.NewEncoder(w).Encode(map[string]string{"error": "分析任務已在進行中，請稍候。"})
		return
	}
	h.isAnalyzing = true
	h.mu.Unlock()

	log.Println("資訊：[TriggerAnalysisHandler] 收到手動觸發影片分析請求，準備啟動 goroutine。")

	go func() {
		defer func() {
			h.mu.Lock()
			h.isAnalyzing = false
			h.mu.Unlock()
			log.Println("資訊：[TriggerAnalysisHandler] 手動觸發的分析任務 goroutine 已結束。")
		}()

		log.Println("資訊：[TriggerAnalysisHandler] 開始執行手動觸發的影片分析任務...")
		err := h.analyzeService.Run()
		if err != nil {
			log.Printf("錯誤：[TriggerAnalysisHandler] 手動觸發的影片分析任務執行失敗: %v", err)
		} else {
			log.Println("資訊：[TriggerAnalysisHandler] 手動觸發的影片分析任務執行成功。")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "影片分析已觸發，正在背景執行。請稍後查看結果。"})
}
