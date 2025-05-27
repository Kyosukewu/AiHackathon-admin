package nas

import (
	"AiHackathon-admin/internal/config" // 引入我們定義的 config 套件
	"fmt"
	"io/ioutil" // 用於讀寫檔案
	"log"
	"os"            // 用於檔案系統操作，如建立目錄、檢查檔案是否存在
	"path/filepath" // 用於處理檔案路徑，確保跨平台相容性
	"time"          // 用於根據日期建立子目錄 (可選)
)

// FileSystemStorage 結構負責與本地檔案系統互動
type FileSystemStorage struct {
	basePath string // 從設定檔讀取的影片儲存根路徑
}

// NewFileSystemStorage 建立一個 FileSystemStorage 實例
// 它會檢查 basePath 是否存在，如果不存在則嘗試建立它。
func NewFileSystemStorage(nasCfg config.NASConfig) (*FileSystemStorage, error) {
	if nasCfg.VideoPath == "" {
		return nil, fmt.Errorf("NAS 設定中的 videoPath 不得為空")
	}

	// 取得絕對路徑，如果 basePath 是相對路徑，則相對於目前工作目錄
	absBasePath, err := filepath.Abs(nasCfg.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("無法取得 NAS videoPath 的絕對路徑 '%s': %w", nasCfg.VideoPath, err)
	}

	// 檢查根目錄是否存在，不存在則建立
	// os.ModePerm (0777) 允許所有權限，您可能需要根據安全性調整
	if _, err := os.Stat(absBasePath); os.IsNotExist(err) {
		log.Printf("資訊：NAS 根目錄 '%s' 不存在，正在嘗試建立...", absBasePath)
		if err := os.MkdirAll(absBasePath, os.ModePerm); err != nil {
			return nil, fmt.Errorf("無法建立 NAS 根目錄 '%s': %w", absBasePath, err)
		}
		log.Printf("資訊：NAS 根目錄 '%s' 建立成功。", absBasePath)
	} else if err != nil {
		return nil, fmt.Errorf("檢查 NAS 根目錄 '%s' 時發生錯誤: %w", absBasePath, err)
	}

	log.Printf("資訊：FileSystemStorage 初始化成功，影片根路徑設定為: %s", absBasePath)
	return &FileSystemStorage{basePath: absBasePath}, nil
}

// buildTargetPath 根據來源名稱、來源ID和原始檔名構造一個建議的儲存路徑
// 例如：/basePath/ap/2025/05/24/source_id_abc/original_filename.mp4
func (fs *FileSystemStorage) buildTargetPath(sourceName, sourceID, originalFileName string) string {
	// 使用日期建立子目錄，有助於組織檔案
	datePath := time.Now().Format("2006/01/02") // 年/月/日

	// 清理 sourceName 和 sourceID，避免路徑問題 (例如，移除特殊字元)
	// 這裡僅作簡單處理，實際情況可能需要更完善的清理函式
	safeSourceName := filepath.Clean(sourceName)
	safeSourceID := filepath.Clean(sourceID) // 確保 sourceID 不包含路徑遍歷字元

	// 組合路徑： basePath / sourceName / datePath / sourceID_原始檔名 (避免檔名衝突)
	// 考慮到 sourceID 可能很長或包含不適合檔名的字元，也可以考慮用 sourceID 當作目錄名
	// 此處範例： basePath / sourceName / datePath / sourceID / originalFileName
	targetDir := filepath.Join(fs.basePath, safeSourceName, datePath, safeSourceID)
	return filepath.Join(targetDir, originalFileName)
}

// SaveVideo 將影片數據儲存到本地檔案系統 (NAS)
// sourceName: 影片來源 (e.g., "ap", "reuters", "youtube", "cnn_nhk_recordings")
// sourceID: 影片在來源系統的唯一 ID (用於建立獨特的檔案名或子目錄)
// originalFileName: 原始影片檔名 (例如 "news_clip.mp4")
// videoData: 影片的二進位數據
// 返回儲存後的相對路徑 (相對於 basePath) 或絕對路徑，以及可能的錯誤
func (fs *FileSystemStorage) SaveVideo(sourceName string, sourceID string, originalFileName string, videoData []byte) (string, error) {
	if sourceName == "" || sourceID == "" || originalFileName == "" {
		return "", fmt.Errorf("SaveVideo 參數 sourceName, sourceID, originalFileName 不得為空")
	}
	if len(videoData) == 0 {
		return "", fmt.Errorf("SaveVideo 參數 videoData 不得為空")
	}

	targetPath := fs.buildTargetPath(sourceName, sourceID, originalFileName)

	// 獲取目標路徑的目錄部分
	targetDir := filepath.Dir(targetPath)

	// 檢查目標目錄是否存在，不存在則建立 (包含所有父目錄)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		log.Printf("資訊：目標目錄 '%s' 不存在，正在嘗試建立...", targetDir)
		if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
			return "", fmt.Errorf("無法建立目標目錄 '%s': %w", targetDir, err)
		}
		log.Printf("資訊：目標目錄 '%s' 建立成功。", targetDir)
	}

	// 寫入檔案
	log.Printf("資訊：正在將影片儲存到 '%s'", targetPath)
	if err := ioutil.WriteFile(targetPath, videoData, 0644); err != nil { // 0644 檔案權限
		return "", fmt.Errorf("無法寫入影片檔案到 '%s': %w", targetPath, err)
	}

	log.Printf("資訊：影片成功儲存到 '%s'", targetPath)

	// 回傳相對於 basePath 的路徑，方便資料庫儲存和後續查找
	relativePath, err := filepath.Rel(fs.basePath, targetPath)
	if err != nil {
		// 如果無法取得相對路徑 (理論上不應該發生，因為 targetPath 是基於 basePath 構造的)
		// 則回傳絕對路徑，並記錄一個警告
		log.Printf("警告：無法取得相對於 basePath '%s' 的相對路徑，將回傳絕對路徑 '%s': %v", fs.basePath, targetPath, err)
		return targetPath, nil
	}

	return relativePath, nil
}

// GetVideoAbsolutePath 根據儲存在資料庫中的相對路徑，取得影片的絕對路徑
func (fs *FileSystemStorage) GetVideoAbsolutePath(relativePath string) (string, error) {
	if relativePath == "" {
		return "", fmt.Errorf("GetVideoAbsolutePath 參數 relativePath 不得為空")
	}
	// 簡單地將 basePath 和 relativePath 組合起來
	// filepath.Join 會處理路徑分隔符
	absPath := filepath.Join(fs.basePath, relativePath)

	// 驗證檔案是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("影片檔案在路徑 '%s' (基於相對路徑 '%s') 上不存在: %w", absPath, relativePath, err)
	} else if err != nil {
		return "", fmt.Errorf("檢查影片檔案 '%s' 時發生錯誤: %w", absPath, err)
	}

	return absPath, nil
}

// ReadVideo 從本地檔案系統 (NAS) 讀取影片內容
// filePathInDB: 儲存在資料庫中的影片路徑 (我們約定這是相對於 basePath 的路徑)
func (fs *FileSystemStorage) ReadVideo(filePathInDB string) ([]byte, error) {
	absolutePath, err := fs.GetVideoAbsolutePath(filePathInDB)
	if err != nil {
		return nil, fmt.Errorf("無法獲取影片絕對路徑: %w", err)
	}

	log.Printf("資訊：正在從 '%s' 讀取影片...", absolutePath)
	videoData, err := ioutil.ReadFile(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("無法讀取影片檔案 '%s': %w", absolutePath, err)
	}
	log.Printf("資訊：影片 '%s' 讀取成功。", absolutePath)
	return videoData, nil
}

// DeleteVideo 從本地檔案系統 (NAS) 刪除影片檔案 (可選功能)
// filePathInDB: 儲存在資料庫中的影片路徑
func (fs *FileSystemStorage) DeleteVideo(filePathInDB string) error {
	absolutePath, err := fs.GetVideoAbsolutePath(filePathInDB)
	if err != nil {
		// 如果檔案本身就不存在於 GetVideoAbsolutePath 階段，可能直接回傳 nil 也是合理的
		// 取決於業務邏輯，這裡假設如果路徑無效則回傳錯誤
		return fmt.Errorf("無法獲取待刪除影片的絕對路徑: %w", err)
	}

	log.Printf("資訊：正在從 '%s' 刪除影片...", absolutePath)
	if err := os.Remove(absolutePath); err != nil {
		return fmt.Errorf("無法刪除影片檔案 '%s': %w", absolutePath, err)
	}
	log.Printf("資訊：影片 '%s' 刪除成功。", absolutePath)

	// (可選) 檢查並刪除空的父目錄
	parentDir := filepath.Dir(absolutePath)
	// 這部分邏輯可以更複雜，例如遞迴刪除直到 basePath 的空目錄
	// 簡單起見，這裡只刪除直接父目錄 (如果它是空的)
	if items, err := ioutil.ReadDir(parentDir); err == nil && len(items) == 0 {
		// 避免刪除 basePath 本身或其直接子目錄 (如 /ap, /reuters)
		// 確保 parentDir 不是 basePath 或 basePath 的一級子目錄
		if parentDir != fs.basePath && filepath.Dir(parentDir) != fs.basePath {
			log.Printf("資訊：嘗試刪除空目錄 '%s'", parentDir)
			os.Remove(parentDir) // 忽略錯誤，因為它不是關鍵操作
		}
	}
	return nil
}
