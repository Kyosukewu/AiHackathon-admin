package scheduler

import (
	"AiHackathon-admin/internal/services"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler 結構 (保持不變)
type Scheduler struct {
	cron       *cron.Cron
	fetchJob   *FetchJob
	analyzeJob *AnalyzeJob
}

// NewScheduler 更新：接收 Cron 表達式
func NewScheduler(
	fs *services.FetchService,
	as *services.AnalyzeService,
	fetchCronSpec string, // 新增參數
	analyzeCronSpec string, // 新增參數
) *Scheduler {
	c := cron.New(cron.WithSeconds())

	fetchJob := NewFetchJob(fs)
	analyzeJob := NewAnalyzeJob(as)

	// 使用從設定檔傳入的 Cron 表達式
	if fetchCronSpec != "" {
		_, err := c.AddJob(fetchCronSpec, fetchJob)
		if err != nil {
			log.Fatalf("錯誤：無法新增影片擷取任務到排程器 (spec: %s): %v", fetchCronSpec, err)
		}
		log.Printf("資訊：影片擷取任務已註冊，排程：%s\n", fetchCronSpec)
	} else {
		log.Println("警告：未提供影片擷取任務的 Cron 表達式，該任務將不會被排程。")
	}

	if analyzeCronSpec != "" {
		_, err := c.AddJob(analyzeCronSpec, analyzeJob)
		if err != nil {
			log.Fatalf("錯誤：無法新增影片分析任務到排程器 (spec: %s): %v", analyzeCronSpec, err)
		}
		log.Printf("資訊：影片分析任務已註冊，排程：%s\n", analyzeCronSpec)
	} else {
		log.Println("警告：未提供影片分析任務的 Cron 表達式，該任務將不會被排程。")
	}

	return &Scheduler{
		cron:       c,
		fetchJob:   fetchJob,
		analyzeJob: analyzeJob,
	}
}

// Start 方法 (不再直接註冊任務，因為已在 NewScheduler 中完成)
func (s *Scheduler) Start() {
	s.cron.Start() // 非阻塞啟動
	log.Println("資訊：排程器已非阻塞啟動 (如果任務已註冊)。")
}

// Stop 方法 (保持不變)
func (s *Scheduler) Stop() {
	log.Println("資訊：正在停止排程器...")
	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
		log.Println("資訊：排程器已優雅停止，所有運行中任務已完成。")
	case <-time.After(10 * time.Second):
		log.Println("警告：排程器停止超時，可能仍有任務在執行。")
	}
}
