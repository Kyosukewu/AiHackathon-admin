package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// --- 新增：VideoAnalysisPrompts 結構 ---
type VideoAnalysisPrompts struct {
	CurrentVersion string            `mapstructure:"currentVersion"`
	Versions       map[string]string `mapstructure:"versions"`
}

// --- 結束新增 ---

// PromptConfig 結構更新：
type PromptConfig struct {
	VideoAnalysis VideoAnalysisPrompts `mapstructure:"videoAnalysis"` // 修改此處
}

// SchedulerConfig (保持不變)
type SchedulerConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	FetchCronSpec   string `mapstructure:"fetchCronSpec"`
	AnalyzeCronSpec string `mapstructure:"analyzeCronSpec"`
}

// Config 結構 (保持不變)
type Config struct {
	AppName       string              `mapstructure:"appName"`
	APClient      APClientConfig      `mapstructure:"apClient"`
	ReutersClient ReutersClientConfig `mapstructure:"reutersClient"`
	YouTubeClient YouTubeClientConfig `mapstructure:"youTubeClient"`
	GeminiClient  GeminiClientConfig  `mapstructure:"geminiClient"`
	Database      DatabaseConfig      `mapstructure:"database"`
	NAS           NASConfig           `mapstructure:"nas"`
	Prompts       PromptConfig        `mapstructure:"prompts"`
	Scheduler     SchedulerConfig     `mapstructure:"scheduler"`
}

// APClientConfig, ReutersClientConfig, YouTubeClientConfig, GeminiClientConfig, DatabaseConfig, NASConfig (保持不變)
// ... (略過)
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
	APIKey string `mapstructure:"apiKey"`
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

// Load 函式 (新增 Prompts 的預設值)
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
	// --- 修改/新增 Prompts 的預設值 ---
	// 提供一個基本的預設 Prompt 和版本
	v.SetDefault("prompts.videoAnalysis.currentVersion", "default-v1")
	v.SetDefault("prompts.videoAnalysis.versions.default-v1", "請分析影片。")
	// --- 結束修改 ---
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

	// (可選) 驗證或記錄重要設定
	// ... (日誌記錄保持不變或根據需要調整)
	if cfg.GeminiClient.APIKey == "" {
		fmt.Println("警告：Gemini API Key 未設定！")
	}
	if !v.IsSet("prompts.videoAnalysis.currentVersion") {
		fmt.Println("警告：Prompt 版本設定使用的是預設值。")
	}

	fmt.Println("資訊：設定載入成功。")
	return &cfg, nil
}
