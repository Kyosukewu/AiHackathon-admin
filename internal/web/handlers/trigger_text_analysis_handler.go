package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// TextAnalysisPipelineRunner 定義了文本分析流程執行者需要的方法
type TextAnalysisPipelineRunner interface {
	ExecuteTextAnalysisPipeline() error
}

// TriggerTextAnalysisHandler 負責處理手動觸發文本元數據分析的請求
type TriggerTextAnalysisHandler struct {
	analyzeService TextAnalysisPipelineRunner // 依賴介面
	mu             sync.Mutex
	isProcessing   bool
}

// NewTriggerTextAnalysisHandler 建立一個 TriggerTextAnalysisHandler 實例
func NewTriggerTextAnalysisHandler(as TextAnalysisPipelineRunner) *TriggerTextAnalysisHandler {
	if as == nil {
		log.Panicln("TriggerTextAnalysisHandler：TextAnalysisPipelineRunner 不得為空")
	}
	return &TriggerTextAnalysisHandler{
		analyzeService: as,
	}
}

// ServeHTTP 實現 http.Handler 介面
func (h *TriggerTextAnalysisHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("資訊：[TriggerTextAnalysisHandler] 收到請求: %s %s 來自 %s\n", r.Method, r.URL.Path, r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Printf("警告：[TriggerTextAnalysisHandler] 收到非 POST 請求 (%s)，已拒絕。\n", r.Method)
		http.Error(w, "僅支援 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	h.mu.Lock()
	if h.isProcessing {
		h.mu.Unlock()
		log.Println("警告：[TriggerTextAnalysisHandler] 文本元數據分析已在進行中，請稍後再試。")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict) // 409 Conflict
		json.NewEncoder(w).Encode(map[string]string{"error": "文本元數據分析任務已在進行中，請稍候。"})
		return
	}
	h.isProcessing = true
	h.mu.Unlock()

	log.Println("資訊：[TriggerTextAnalysisHandler] 收到手動觸發文本元數據分析請求，準備啟動 goroutine。")

	go func() {
		defer func() {
			h.mu.Lock()
			h.isProcessing = false
			h.mu.Unlock()
			log.Println("資訊：[TriggerTextAnalysisHandler] 手動觸發的文本元數據分析任務 goroutine 已結束。")
		}()

		log.Println("資訊：[TriggerTextAnalysisHandler] 開始執行手動觸發的文本元數據分析任務...")
		err := h.analyzeService.ExecuteTextAnalysisPipeline()
		if err != nil {
			log.Printf("錯誤：[TriggerTextAnalysisHandler] 手動觸發的文本元數據分析任務執行失敗: %v", err)
		} else {
			log.Println("資訊：[TriggerTextAnalysisHandler] 手動觸發的文本元數據分析任務執行成功。")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "文本元數據分析已觸發，正在背景執行。"})
}
