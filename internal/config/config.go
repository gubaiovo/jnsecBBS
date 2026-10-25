package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Database struct {
	Driver string `yaml:"driver"` // sqlite | mysql
	DSN    string `yaml:"dsn"`
}

type Config struct {
	ReadOnly      bool   `yaml:"read_only"`
	Listen        string `yaml:"listen"`
	SiteName      string `yaml:"site_name"`
	Timezone      string `yaml:"timezone"`
	DataDir       string `yaml:"data_dir"`
	SessionSecret string `yaml:"session_secret"`
	CookieSecure  bool   `yaml:"cookie_secure"`
	TrustProxy    bool   `yaml:"trust_proxy"`
	// TrustedOrigins 是 gorilla/csrf 同源校验的白名单（完整 Origin，含协议）。
	// TLS 在 nginx 终止、应用收到明文 HTTP 时，Origin 与转发 Host 不一致会误杀写请求，白名单兜底。
	TrustedOrigins []string `yaml:"trusted_origins"`
	Database       Database `yaml:"database"`
	Registration   string   `yaml:"registration"` // open | invite
	LogLevel       string   `yaml:"log_level"`
	AssetsDir      string   `yaml:"assets_dir"` // 旧 Flarum 静态资源目录（/assets/* 头像与附件继续可访问）
}

func defaults() *Config {
	return &Config{
		Listen:        "0.0.0.0:8080",
		SiteName:      "JNSEC BBS",
		Timezone:      "Asia/Shanghai",
		DataDir:       "./data",
		SessionSecret: "",
		CookieSecure:  false,
		TrustProxy:    false,
		Database:      Database{Driver: "sqlite", DSN: "./bbs.db"},
		Registration:  "invite",
		LogLevel:      "info",
	}
}

// Load 读取 YAML 配置；文件缺失或字段留空时回退默认值，
// 最后应用 JNSEC_* 环境变量覆盖（Docker/Compose 部署无需挂载配置文件）。
func Load(path string) (*Config, error) {
	cfg := defaults()
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, err
	}
	applyEnv(cfg)
	return cfg, nil
}

// applyEnv 环境变量优先级最高，便于容器化部署。
func applyEnv(cfg *Config) {
	if v := os.Getenv("JNSEC_READ_ONLY"); v != "" {
		cfg.ReadOnly = truthy(v)
	}
	if v := os.Getenv("JNSEC_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("JNSEC_SITE_NAME"); v != "" {
		cfg.SiteName = v
	}
	if v := os.Getenv("JNSEC_TIMEZONE"); v != "" {
		cfg.Timezone = v
	}
	if v := os.Getenv("JNSEC_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("JNSEC_SESSION_SECRET"); v != "" {
		cfg.SessionSecret = v
	}
	if v := os.Getenv("JNSEC_REGISTRATION"); v != "" {
		cfg.Registration = v
	}
	if v := os.Getenv("JNSEC_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("JNSEC_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("JNSEC_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("JNSEC_COOKIE_SECURE"); v != "" {
		cfg.CookieSecure = truthy(v)
	}
	if v := os.Getenv("JNSEC_TRUST_PROXY"); v != "" {
		cfg.TrustProxy = truthy(v)
	}
	if v := os.Getenv("JNSEC_TRUSTED_ORIGINS"); v != "" {
		cfg.TrustedOrigins = strings.Split(strings.ReplaceAll(v, " ", ""), ",")
	}
	if v := os.Getenv("JNSEC_ASSETS_DIR"); v != "" {
		cfg.AssetsDir = v
	}
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
