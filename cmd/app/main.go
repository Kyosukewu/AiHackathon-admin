package main

import (
	"AiHackathon-admin/internal/clients/gemini"
	"AiHackathon-admin/internal/config"
	"AiHackathon-admin/internal/scheduler"
	"AiHackathon-admin/internal/services"
	"AiHackathon-admin/internal/storage/mysql"
	"AiHackathon-admin/internal/storage/nas"
	"AiHackathon-admin/internal/web"
	"AiHackathon-admin/internal/web/handlers"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load("./configs", "config")
	if err != nil {
		log.Fatalf("錯誤：無法載入設定: %v", err)
	}
	log.Println("資訊：應用程式設定載入成功。")

	// 資料庫遷移
	migrationPath := "file://scripts/migrate/mysql"
	dbDSNForMigrate := fmt.Sprintf("mysql://%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
		cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
	log.Printf("資訊：準備執行資料庫遷移，來源: %s, DSN 使用資料庫: %s", migrationPath, cfg.Database.DBName)
	m, err := migrate.New(migrationPath, dbDSNForMigrate)
	if err != nil {
		log.Fatalf("錯誤：建立遷移實例失敗: %v", err)
	}
	currentVersion, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Fatalf("錯誤：獲取資料庫遷移版本失敗: %v", err)
	}
	if dirty {
		log.Fatalf("錯誤：資料庫處於 dirty 狀態 (版本 %d)，遷移失敗。", currentVersion)
	}
	log.Printf("資訊：目前資料庫版本: %d。開始應用遷移...", currentVersion)
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		log.Fatalf("錯誤：執行資料庫遷移 (m.Up) 失敗: %v", err)
	} else if err == migrate.ErrNoChange {
		log.Println("資訊：資料庫結構已是最新，無需遷移。")
	} else {
		newVersion, _, _ := m.Version()
		log.Printf("資訊：資料庫遷移成功完成，版本更新至: %d。", newVersion)
	}

	nasStorage, err := nas.NewFileSystemStorage(cfg.NAS)
	if err != nil {
		log.Fatalf("錯誤：初始化 NAS 儲存失敗: %v", err)
	}

	var dbStore handlers.DBStore
	realDBStore, err := mysql.NewMySQLStore(cfg.Database)
	if err != nil {
		log.Fatalf("錯誤：初始化 MySQL 資料庫連線失敗: %v", err)
	}
	dbStore = realDBStore
	defer realDBStore.Close()

	// 初始化 Gemini 客戶端時傳入模型名稱
	// 您可以將這些模型名稱也移到 config.yaml 中
	textModelName := "gemini-1.5-flash-latest"  // 或者 cfg.GeminiClient.TextModel
	videoModelName := "gemini-1.5-flash-latest" // 或者 cfg.GeminiClient.VideoModel
	geminiClient, err := gemini.NewClient(cfg.GeminiClient.APIKey, textModelName, videoModelName)
	if err != nil {
		log.Fatalf("錯誤：初始化 Gemini 客戶端失敗: %v", err)
	}

	var nasForService services.NASStorage = nasStorage
	fetchSvc, err := services.NewFetchService(cfg, dbStore, nasForService)
	if err != nil {
		log.Fatalf("錯誤：初始化影片擷取服務失敗: %v", err)
	}
	analyzeSvc, err := services.NewAnalyzeService(cfg, dbStore, nasForService, geminiClient)
	if err != nil {
		log.Fatalf("錯誤：初始化影片分析服務失敗: %v", err)
	}

	if cfg.Scheduler.Enabled {
		log.Println("資訊：排程器已在設定檔中啟用，正在初始化...")
		appScheduler := scheduler.NewScheduler(
			fetchSvc,
			analyzeSvc,
			cfg.Scheduler.FetchCronSpec,
			cfg.Scheduler.AnalyzeCronSpec,
		)
		appScheduler.Start()
		log.Println("資訊：排程器已啟動。")
		defer appScheduler.Stop()
	} else {
		log.Println("資訊：排程器已在設定檔中禁用。")
	}

	router := web.SetupRouter(cfg, dbStore, analyzeSvc) // 傳遞 analyzeSvc 給路由
	serverAddr := ":8080"
	server := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	go func() {
		log.Printf("資訊：HTTP 伺服器正在監聽 %s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("錯誤：HTTP 伺服器監聽失敗: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("資訊：收到關閉訊號，正在關閉應用程式...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("錯誤：HTTP 伺服器優雅關閉失敗: %v", err)
	}
	log.Println("資訊：HTTP 伺服器已關閉。")
	log.Println("資訊：應用程式已成功關閉。")
}
