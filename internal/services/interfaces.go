package services

// NASStorage 介面定義了儲存操作
type NASStorage interface {
	SaveVideo(sourceName string, sourceID string, originalFileName string, videoData []byte) (string, error)
	GetVideoAbsolutePath(relativePath string) (string, error)
	ReadVideo(filePath string) ([]byte, error)
	// DeleteVideo(filePathInDB string) error // 如果需要
}

// 您也可以將 DBStore 介面從 handlers 移到這裡，如果它主要由 services 層使用
// type DBStore interface { ... }
