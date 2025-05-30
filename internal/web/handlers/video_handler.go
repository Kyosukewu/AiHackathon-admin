package handlers

import (
	"AiHackathon-admin/internal/config" // 引入 config 以獲取 NAS 路徑
	"fmt"
	"log"
	"net/http"
	"os"            // 用於檢查檔案是否存在
	"path/filepath" // 用於安全地處理檔案路徑
	"strings"
)

// VideoHandler 負責提供影片檔案串流
type VideoHandler struct {
	nasBasePath string // NAS 影片儲存的絕對根路徑
}

// NewVideoHandler 建立一個 VideoHandler 實例
func NewVideoHandler(nasCfg config.NASConfig) (*VideoHandler, error) {
	if nasCfg.VideoPath == "" {
		return nil, fmt.Errorf("VideoHandler: NAS 設定中的 videoPath 不得為空")
	}
	absBasePath, err := filepath.Abs(nasCfg.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("VideoHandler: 無法取得 NAS videoPath 的絕對路徑 '%s': %w", nasCfg.VideoPath, err)
	}
	log.Printf("資訊：[VideoHandler] 初始化成功，影片服務根路徑: %s", absBasePath)
	return &VideoHandler{nasBasePath: absBasePath}, nil
}

// ServeHTTP 實現 http.Handler 介面
// 它期望 URL 路徑是 /media/{影片在NASPath下的相對路徑}
// 例如：/media/ap/videoID123/video.mp4
func (h *VideoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 從 URL 中提取影片的相對路徑
	// URL.Path 會是例如 "/media/ap/videoID123/video.mp4"
	// 我們需要移除 "/media/" 前綴
	relativePath := strings.TrimPrefix(r.URL.Path, "/media/")
	if relativePath == "" || strings.HasSuffix(relativePath, "/") {
		http.Error(w, "無效的影片路徑", http.StatusBadRequest)
		return
	}

	// 安全地組合絕對路徑
	// filepath.Join 會清理路徑，防止路徑遍歷攻擊 (例如 ../)
	// 並確保使用的是作業系統正確的路徑分隔符
	fullPath := filepath.Join(h.nasBasePath, relativePath)

	// 再次清理，並檢查最終路徑是否仍然在 basePath 下，防止惡意遍歷
	cleanedFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		log.Printf("錯誤：[VideoHandler] 無法解析影片絕對路徑 '%s': %v", fullPath, err)
		http.Error(w, "內部伺服器錯誤", http.StatusInternalServerError)
		return
	}
	if !strings.HasPrefix(cleanedFullPath, h.nasBasePath) {
		log.Printf("警告：[VideoHandler] 偵測到潛在的路徑遍歷嘗試: '%s' (解析為 '%s')", relativePath, cleanedFullPath)
		http.Error(w, "禁止存取", http.StatusForbidden)
		return
	}

	// 檢查檔案是否存在
	if _, err := os.Stat(cleanedFullPath); os.IsNotExist(err) {
		log.Printf("錯誤：[VideoHandler] 請求的影片檔案不存在: %s", cleanedFullPath)
		http.NotFound(w, r)
		return
	} else if err != nil {
		log.Printf("錯誤：[VideoHandler] 檢查影片檔案 '%s' 時發生錯誤: %v", cleanedFullPath, err)
		http.Error(w, "內部伺服器錯誤", http.StatusInternalServerError)
		return
	}

	log.Printf("資訊：[VideoHandler] 正在提供影片: %s", cleanedFullPath)
	// http.ServeFile 會自動處理 Content-Type, ETag, Range requests (用於影片跳轉) 等
	http.ServeFile(w, r, cleanedFullPath)
}
