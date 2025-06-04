package handlers

import (
	"AiHackathon-admin/internal/models"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ExportHandler 負責處理匯出請求
type ExportHandler struct {
	db DBStore
}

// NewExportHandler 建立一個 ExportHandler 實例
func NewExportHandler(db DBStore) *ExportHandler {
	if db == nil {
		log.Panicln("ExportHandler：DBStore 不得為空")
	}
	return &ExportHandler{
		db: db,
	}
}

// ServeHTTP 實現 http.Handler 介面
func (h *ExportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("資訊：[ExportHandler] 收到請求: %s %s 來自 %s\n", r.Method, r.URL.Path, r.RemoteAddr)

	if r.Method != http.MethodGet {
		log.Printf("警告：[ExportHandler] 收到非 GET 請求 (%s)，已拒絕。\n", r.Method)
		http.Error(w, "僅支援 GET 方法", http.StatusMethodNotAllowed)
		return
	}

	// 從 URL 查詢參數讀取篩選和排序條件
	searchTerm := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")

	// 設定預設排序
	if sortBy == "" {
		sortBy = "importance"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// 獲取所有影片資料（不分頁）
	videos, analysisResults, err := h.db.GetAllVideosWithAnalysis(1000, 0, searchTerm, sortBy, sortOrder)
	if err != nil {
		log.Printf("錯誤：[ExportHandler] 從資料庫獲取影片數據失敗: %v", err)
		http.Error(w, "無法獲取匯出數據", http.StatusInternalServerError)
		return
	}

	// 新增除錯資訊
	log.Printf("資訊：[ExportHandler] 獲取到 %d 個影片和 %d 個分析結果", len(videos), len(analysisResults))

	// 建立分析結果映射
	analysisResultMap := make(map[int64]models.AnalysisResult)
	for _, ar := range analysisResults {
		analysisResultMap[ar.VideoID] = ar
		log.Printf("資訊：[ExportHandler] 分析結果 VideoID: %d, 包含資料：Transcript: %v, ShortSummary: %v, BulletedSummary: %v",
			ar.VideoID,
			ar.Transcript != nil && ar.Transcript.Valid,
			ar.ShortSummary != nil && ar.ShortSummary.Valid,
			ar.BulletedSummary != nil && ar.BulletedSummary.Valid)
	}

	// 設定 CSV 檔案標頭
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=影片分析資料_%s.csv", time.Now().Format("2006-01-02")))

	// 建立 CSV writer
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// 寫入標題列
	headers := []string{
		"素材編號",
		"大標題",
		"發布時間",
		"短摘要",
		"列點摘要",
		"畫面",
		"BITE",
		"長度",
		"地點",
		"重要性評分",
		"關鍵字",
		"分類",
		"素材類型",
		"標來源",
		"原始逐字稿",
		"原始翻譯",
		"原始畫面",
	}
	if err := writer.Write(headers); err != nil {
		log.Printf("錯誤：[ExportHandler] 寫入 CSV 標題失敗: %v", err)
		return
	}

	// 寫入資料列
	for _, v := range videos {
		ar, hasAnalysis := analysisResultMap[v.ID]

		// 準備資料
		row := make([]string, len(headers))
		row[0] = fmt.Sprintf("%s%s", v.SourceName, v.SourceID) // 素材編號
		row[1] = v.Title.String                                // 大標題
		if v.PublishedAt.Valid {
			row[2] = v.PublishedAt.Time.Format("2006-01-02 15:04:05") // 發布時間
		}
		if hasAnalysis {
			log.Printf("資訊：[ExportHandler] 處理影片 ID: %d 的分析結果", v.ID)
			if ar.ShortSummary != nil && ar.ShortSummary.Valid {
				row[3] = ar.ShortSummary.String // 短摘要
				log.Printf("資訊：[ExportHandler] 影片 ID: %d 的短摘要: %s", v.ID, ar.ShortSummary.String)
			}
			if ar.BulletedSummary != nil && ar.BulletedSummary.Valid {
				row[4] = ar.BulletedSummary.String // 列點摘要
				log.Printf("資訊：[ExportHandler] 影片 ID: %d 的列點摘要: %s", v.ID, ar.BulletedSummary.String)
			}
			if ar.Transcript != nil && ar.Transcript.Valid {
				row[14] = ar.Transcript.String // 原始逐字稿
				log.Printf("資訊：[ExportHandler] 影片 ID: %d 的逐字稿長度: %d", v.ID, len(ar.Transcript.String))
			}
			if ar.Translation != nil && ar.Translation.Valid {
				row[15] = ar.Translation.String // 原始翻譯
				log.Printf("資訊：[ExportHandler] 影片 ID: %d 的翻譯長度: %d", v.ID, len(ar.Translation.String))
			}
			if ar.VisualDescription != nil && ar.VisualDescription.Valid {
				row[16] = ar.VisualDescription.String // 原始畫面
				log.Printf("資訊：[ExportHandler] 影片 ID: %d 的畫面描述長度: %d", v.ID, len(ar.VisualDescription.String))
			}
			if ar.MaterialType != nil && ar.MaterialType.Valid {
				row[12] = ar.MaterialType.String // 素材類型
				log.Printf("資訊：[ExportHandler] 影片 ID: %d 的素材類型: %s", v.ID, ar.MaterialType.String)
			}
		} else {
			log.Printf("警告：[ExportHandler] 影片 ID: %d 沒有分析結果", v.ID)
		}
		if v.ShotlistContent.Valid {
			row[5] = v.ShotlistContent.String // 畫面
		}
		if hasAnalysis && len(ar.Bites) > 0 {
			var bites []string
			var bitesData []struct {
				TimeLine string `json:"time_line"`
				Quote    string `json:"quote"`
			}
			if err := json.Unmarshal(ar.Bites, &bitesData); err == nil {
				for _, bite := range bitesData {
					bites = append(bites, fmt.Sprintf("%s: %s", bite.TimeLine, bite.Quote))
				}
			}
			row[6] = strings.Join(bites, "; ") // BITE
		}
		if v.DurationSecs.Valid {
			row[7] = fmt.Sprintf("%02d:%02d", v.DurationSecs.Int64/60, v.DurationSecs.Int64%60) // 長度
		}
		row[8] = v.Location.String // 地點
		if hasAnalysis && ar.ImportanceScore != nil {
			var importanceData struct {
				OverallRating string `json:"overall_rating"`
			}
			if err := json.Unmarshal(ar.ImportanceScore, &importanceData); err == nil {
				row[9] = importanceData.OverallRating // 重要性評分
			}
		}
		if hasAnalysis && len(ar.Keywords) > 0 {
			var keywords []string
			var keywordsData []struct {
				Category string `json:"category"`
				Keyword  string `json:"keyword"`
			}
			if err := json.Unmarshal(ar.Keywords, &keywordsData); err == nil {
				for _, kw := range keywordsData {
					keywords = append(keywords, fmt.Sprintf("%s: %s", kw.Category, kw.Keyword))
				}
			}
			row[10] = strings.Join(keywords, "; ") // 關鍵字
		}
		if len(v.Subjects) > 0 {
			var subjects []string
			if err := json.Unmarshal(v.Subjects, &subjects); err == nil {
				row[11] = strings.Join(subjects, "; ") // 分類
			}
		}
		row[13] = fmt.Sprintf("%s%s", v.SourceName, v.SourceID) // 標來源

		// 寫入資料列
		if err := writer.Write(row); err != nil {
			log.Printf("錯誤：[ExportHandler] 寫入 CSV 資料列失敗: %v", err)
			return
		}
	}
}
