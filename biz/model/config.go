package model

import "os"

type Config struct {
	DB     DatabaseConfig
	Redis  RedisConfig
	JWT    JWTConfig
	Server ServerConfig
	AI     AIConfig
}

type AIConfig struct {
	APIKey       string
	Model        string
	BaseURL      string
	MaxTokens    int
	Temperature  float64
	SystemPrompt string
}

// 国内免费 AI 服务商配置参考（任选其一）
// 1. DeepSeek: https://platform.deepseek.com/
//   - API Key: 在平台获取
//   - BaseURL: https://api.deepseek.com
//   - 推荐模型: deepseek-chat
//
// 2. 智谱AI (GLM): https://open.bigmodel.cn/
//   - API Key: 在平台获取
//   - BaseURL: https://open.bigmodel.cn/api/paas/v4
//   - 推荐模型: GLM-4-Flash
//
// 3. 豆包 (火山引擎): https://console.volcengine.com/ark/
//   - API Key: 在平台获取
//   - BaseURL: https://ark.cn-beijing.volces.com/api/v3
//   - 推荐模型: ep-20241108... (自行创建推理接入点)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
}

type ServerConfig struct {
	Port string
}

func LoadConfig() *Config {
	return &Config{
		DB: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", "root"),
			Name:     getEnv("DB_NAME", "video_website"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
		JWT: JWTConfig{
			AccessSecret:  getEnv("JWT_ACCESS_SECRET", "your-access-secret-key"),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", "your-refresh-secret-key"),
		},
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8888"),
		},
		AI: AIConfig{
			APIKey:       getEnv("AI_API_KEY", ""),
			Model:        getEnv("AI_MODEL", "deepseek-chat"),
			BaseURL:      getEnv("AI_BASE_URL", "https://api.deepseek.com"),
			MaxTokens:    1024,
			Temperature:  0.7,
			SystemPrompt: getEnv("AI_SYSTEM_PROMPT", "你是一个友好的聊天助手，名叫FanAI。你会在用户聊天时适时加入对话，提供有趣的观点和建议。请用简洁自然的语言回复，不要过长。"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
