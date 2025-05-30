package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// VideoAnalysisPrompts 用於存放影片分析相關的 Prompt 版本和當前使用版本
type VideoAnalysisPrompts struct {
	CurrentVersion string            `mapstructure:"currentVersion"`
	Versions       map[string]string `mapstructure:"versions"`
}

// TextFileAnalysisPrompts 用於存放文本檔案分析相關的 Prompt 版本和當前使用版本
type TextFileAnalysisPrompts struct {
	CurrentVersion string            `mapstructure:"currentVersion"`
	Versions       map[string]string `mapstructure:"versions"`
}

// PromptConfig 結構用於組織所有類型的 Prompt 設定
type PromptConfig struct {
	VideoAnalysis    VideoAnalysisPrompts    `mapstructure:"videoAnalysis"`
	TextFileAnalysis TextFileAnalysisPrompts `mapstructure:"textFileAnalysis"` // <--- 確保此欄位存在
}

// SchedulerConfig 用於存放排程器相關設定
type SchedulerConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	FetchCronSpec   string `mapstructure:"fetchCronSpec"`
	AnalyzeCronSpec string `mapstructure:"analyzeCronSpec"`
}

// Config 結構 (主設定結構)
type Config struct {
	AppName       string              `mapstructure:"appName"`
	APClient      APClientConfig      `mapstructure:"apClient"`
	ReutersClient ReutersClientConfig `mapstructure:"reutersClient"`
	YouTubeClient YouTubeClientConfig `mapstructure:"youTubeClient"`
	GeminiClient  GeminiClientConfig  `mapstructure:"geminiClient"`
	Database      DatabaseConfig      `mapstructure:"database"`
	NAS           NASConfig           `mapstructure:"nas"`
	Prompts       PromptConfig        `mapstructure:"prompts"`   // 包含所有 Prompt 設定
	Scheduler     SchedulerConfig     `mapstructure:"scheduler"` // 包含排程器設定
}

// APClientConfig AP API 相關設定
type APClientConfig struct {
	APIKey  string `mapstructure:"apiKey"`
	BaseURL string `mapstructure:"baseURL"`
}

// ReutersClientConfig Reuters API 相關設定
type ReutersClientConfig struct {
	ClientID     string `mapstructure:"clientID"`
	ClientSecret string `mapstructure:"clientSecret"`
	Audience     string `mapstructure:"audience"`
	TokenURL     string `mapstructure:"tokenURL"`
	BaseURL      string `mapstructure:"baseURL"`
}

// YouTubeClientConfig YouTube API 相關設定
type YouTubeClientConfig struct {
	APIKey  string `mapstructure:"apiKey"`
	BaseURL string `mapstructure:"baseURL"`
}

// GeminiClientConfig Gemini API 相關設定
type GeminiClientConfig struct {
	APIKey         string `mapstructure:"apiKey"`
	TextModelName  string `mapstructure:"textModelName"`  // 可選，用於文本分析的模型
	VideoModelName string `mapstructure:"videoModelName"` // 可選，用於影片分析的模型
}

// DatabaseConfig 資料庫連線設定
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbName"`
}

// NASConfig NAS 相關設定
type NASConfig struct {
	VideoPath string `mapstructure:"videoPath"`
}

// Load 函式使用 Viper 載入設定
func Load(configPath string, configName string) (*Config, error) {
	v := viper.New()

	v.AddConfigPath(configPath)
	v.SetConfigName(configName)
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 設定預設值
	v.SetDefault("appName", "AiHackathon-DefaultApp")
	v.SetDefault("database.port", 3306)
	v.SetDefault("database.host", "127.0.0.1")
	v.SetDefault("geminiClient.textModelName", "gemini-1.5-flash-latest")  // 預設文本模型
	v.SetDefault("geminiClient.videoModelName", "gemini-1.5-flash-latest") // 預設影片模型

	v.SetDefault("prompts.videoAnalysis.currentVersion", "default-v-fallback")
	v.SetDefault("prompts.videoAnalysis.versions.default-v-fallback", "請分析此影片的內容。")
	v.SetDefault("prompts.textFileAnalysis.currentVersion", "default-t-fallback")
	v.SetDefault("prompts.textFileAnalysis.versions.default-t-fallback", "請從文本中提取標題和摘要。")

	v.SetDefault("scheduler.enabled", true)
	v.SetDefault("scheduler.fetchCronSpec", "0 0 * * * *")
	v.SetDefault("scheduler.analyzeCronSpec", "0 */10 * * * *")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("警告：找不到設定檔，將使用預設值和環境變數。")
		} else {
			return nil, fmt.Errorf("讀取設定檔時發生錯誤: %w", err)
		}
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("無法解析設定檔到結構: %w", err)
	}

	// 驗證或記錄重要設定
	if cfg.GeminiClient.APIKey == "" {
		fmt.Println("警告：Gemini API Key 未在設定中提供！")
	}
	if !v.IsSet("prompts.videoAnalysis.currentVersion") {
		fmt.Println("警告：VideoAnalysis Prompt 版本設定使用的是預設值 (因為設定檔中未找到或未透過環境變數設定)。")
	}
	if !v.IsSet("prompts.textFileAnalysis.currentVersion") {
		fmt.Println("警告：TextFileAnalysis Prompt 版本設定使用的是預設值 (因為設定檔中未找到或未透過環境變數設定)。")
	}

	fmt.Println("資訊：設定載入成功。")
	return &cfg, nil
}
