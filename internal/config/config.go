package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

// VideoAnalysisPrompts 結構 (保持不變)
type VideoAnalysisPrompts struct {
	CurrentVersion string            `mapstructure:"currentVersion"`
	Versions       map[string]string `mapstructure:"versions"` // 這裡的 string 現在是檔案路徑
}

// TextFileAnalysisPrompts 結構 (保持不變)
type TextFileAnalysisPrompts struct {
	CurrentVersion string            `mapstructure:"currentVersion"`
	Versions       map[string]string `mapstructure:"versions"` // 這裡的 string 現在是檔案路徑
}

// PromptConfig 結構 (保持不變)
type PromptConfig struct {
	VideoAnalysis    VideoAnalysisPrompts    `mapstructure:"videoAnalysis"`
	TextFileAnalysis TextFileAnalysisPrompts `mapstructure:"textFileAnalysis"`
}

// SchedulerConfig, Config, APClientConfig, etc. (保持不變)
type SchedulerConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	FetchCronSpec   string `mapstructure:"fetchCronSpec"`
	AnalyzeCronSpec string `mapstructure:"analyzeCronSpec"`
}
type Config struct {
	AppName       string
	APClient      APClientConfig
	ReutersClient ReutersClientConfig
	YouTubeClient YouTubeClientConfig
	GeminiClient  GeminiClientConfig
	Database      DatabaseConfig
	NAS           NASConfig
	Prompts       PromptConfig
	Scheduler     SchedulerConfig
}
type APClientConfig struct {
	APIKey  string `mapstructure:"apiKey"`
	BaseURL string `mapstructure:"baseURL"`
}
type ReutersClientConfig struct {
	ClientID     string `mapstructure:"clientID"`
	ClientSecret string `mapstructure:"clientSecret"`
	Audience     string `mapstructure:"audience"`
	TokenURL     string `mapstructure:"tokenURL"`
	BaseURL      string `mapstructure:"baseURL"`
}
type YouTubeClientConfig struct {
	APIKey  string `mapstructure:"apiKey"`
	BaseURL string `mapstructure:"baseURL"`
}
type GeminiClientConfig struct {
	APIKey         string `mapstructure:"apiKey"`
	TextModelName  string `mapstructure:"textModelName"`
	VideoModelName string `mapstructure:"videoModelName"`
}
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbName"`
}
type NASConfig struct {
	VideoPath string `mapstructure:"videoPath"`
}

// Load 函式 (調整 Prompt 的預設值邏輯)
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
	v.SetDefault("geminiClient.textModelName", "gemini-1.5-flash-latest")
	v.SetDefault("geminiClient.videoModelName", "gemini-1.5-flash-latest")

	// 對於 Prompt 路徑，可以設定預設的 currentVersion，但 versions 的路徑如果不存在，
	// 應該由服務層在讀取檔案時處理。
	// 或者，如果希望有一個預設的 prompt 檔案，可以在這裡設定路徑。
	// 為了簡單，我們先不為 versions map 中的路徑設定預設值。
	// 如果 currentVersion 指向的 key 在 versions map 中不存在，服務層會發現。
	v.SetDefault("prompts.videoAnalysis.currentVersion", "default-v-not-found") // 一個標示性的預設版本
	v.SetDefault("prompts.textFileAnalysis.currentVersion", "default-t-not-found")

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

	if cfg.GeminiClient.APIKey == "" {
		fmt.Println("警告：Gemini API Key 未在設定中提供！")
	}
	// 檢查 prompts 的 currentVersion 是否被設定
	if !v.IsSet("prompts.videoAnalysis.currentVersion") {
		log.Println("警告：[Config] VideoAnalysis currentVersion 未在設定檔中設定，將使用預設或可能導致錯誤。")
	} else if _, ok := cfg.Prompts.VideoAnalysis.Versions[cfg.Prompts.VideoAnalysis.CurrentVersion]; !ok {
		log.Printf("警告：[Config] VideoAnalysis currentVersion '%s' 在 versions 中未找到對應的 Prompt 路徑！", cfg.Prompts.VideoAnalysis.CurrentVersion)
	}
	if !v.IsSet("prompts.textFileAnalysis.currentVersion") {
		log.Println("警告：[Config] TextFileAnalysis currentVersion 未在設定檔中設定，將使用預設或可能導致錯誤。")
	} else if _, ok := cfg.Prompts.TextFileAnalysis.Versions[cfg.Prompts.TextFileAnalysis.CurrentVersion]; !ok {
		log.Printf("警告：[Config] TextFileAnalysis currentVersion '%s' 在 versions 中未找到對應的 Prompt 路徑！", cfg.Prompts.TextFileAnalysis.CurrentVersion)
	}

	fmt.Println("資訊：設定載入成功。")
	return &cfg, nil
}
