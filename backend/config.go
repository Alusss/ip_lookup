package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type WebhookConfig struct {
	URL          string             `yaml:"url"`
	SendResolved *bool              `yaml:"send_resolved,omitempty"`
	HTTPConfig   *WebhookHTTPConfig `yaml:"http_config,omitempty"`
}

type WebhookHTTPConfig struct {
	Authorization *WebhookAuthConfig `yaml:"authorization,omitempty"`
}

type WebhookAuthConfig struct {
	Type        string `yaml:"type"`
	Credentials string `yaml:"credentials"`
}

func (wc *WebhookConfig) ShouldSendResolved() bool {
	if wc.SendResolved == nil {
		return true
	}
	return *wc.SendResolved
}

type MonitoringConfig struct {
	Enabled                   bool            `yaml:"enabled"`
	CheckInterval             time.Duration   `yaml:"check_interval"`
	AlertCooldown             time.Duration   `yaml:"alert_cooldown"`
	WebhookConfigs            []WebhookConfig `yaml:"webhook_configs,omitempty"`
	ErrorRateThreshold        float64         `yaml:"error_rate_threshold"`
	P99LatencyThresholdMs     int64           `yaml:"p99_latency_threshold_ms"`
	RateLimitHitRateThreshold float64         `yaml:"rate_limit_hit_rate_threshold"`
}

type ConfigValues struct {
	ListenAddrV4 string `yaml:"listen_addr_v4"`
	ListenAddrV6 string `yaml:"listen_addr_v6"`
	PortV4       int    `yaml:"port_v4"`
	PortV6       int    `yaml:"port_v6"`

	RateEnabled         bool          `yaml:"rate_enabled"`
	RatePerIP           int           `yaml:"rate_per_ip"`
	RatePerIPBurst      int           `yaml:"rate_per_ip_burst"`
	RateGlobal          int           `yaml:"rate_global"`
	RateGlobalBurst     int           `yaml:"rate_global_burst"`
	RateMode            string        `yaml:"rate_mode"`
	RateCleanupInterval time.Duration `yaml:"rate_cleanup_interval"`

	ApiAdEnabled bool   `yaml:"api_ad_enabled"`
	ApiAdTextZh  string `yaml:"api_ad_text_zh"`
	ApiAdUrlZh   string `yaml:"api_ad_url_zh"`
	ApiAdTextEn  string `yaml:"api_ad_text_en"`
	ApiAdUrlEn   string `yaml:"api_ad_url_en"`

	WebAdEnabled bool   `yaml:"web_ad_enabled"`
	WebAdTextZh  string `yaml:"web_ad_text_zh"`
	WebAdUrlZh   string `yaml:"web_ad_url_zh"`
	WebAdTextEn  string `yaml:"web_ad_text_en"`
	WebAdUrlEn   string `yaml:"web_ad_url_en"`

	LogLevel       string `yaml:"log_level"`
	LogFileMaxSize int    `yaml:"log_file_max_size"`
	LogFileMaxAge  int    `yaml:"log_file_max_age"`
	LogFileBackups int    `yaml:"log_file_backups"`
	LogIpMasking   bool   `yaml:"log_ip_masking"`

	CorsEnabled    bool `yaml:"cors_enabled"`
	JsonApiEnabled bool `yaml:"json_api_enabled"`
	AllApiEnabled  bool `yaml:"all_api_enabled"`

	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	MaxHeaderBytes    int           `yaml:"max_header_bytes"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	MaxConnsPerIP     int           `yaml:"max_conns_per_ip"`

	TrustedProxyCIDRs string `yaml:"trusted_proxy_cidrs"`
	IPDenylist        string `yaml:"ip_denylist"`
	UADenylist        string `yaml:"ua_denylist"`

	CfCidrPath           string        `yaml:"cf_cidr_path"`
	CfCidrReloadInterval time.Duration `yaml:"cf_cidr_reload_interval"`
	CfOnly               bool          `yaml:"cf_only"`

	GeoipEnabled   bool   `yaml:"geoip_enabled"`
	GeoipDbPath    string `yaml:"geoip_db_path"`
	GeoipAsnDbPath string `yaml:"geoip_asn_db_path"`

	MetricsListenAddr string `yaml:"metrics_listen_addr"`

	Monitoring MonitoringConfig `yaml:"monitoring"`
}

type Config struct {
	ConfigValues `yaml:",inline"`

	configPath string
	mu         sync.RWMutex
	watcher    *fsnotify.Watcher
	onReload   func(*Config)
}

func DefaultConfig() *Config {
	return &Config{
		ConfigValues: ConfigValues{
			ListenAddrV4: "127.0.0.1",
			ListenAddrV6: "::1",
			PortV4:       8080,
			PortV6:       8081,

			RateEnabled:         true,
			RatePerIP:           10,
			RatePerIPBurst:      5,
			RateGlobal:          5000,
			RateGlobalBurst:     5000,
			RateMode:            "both",
			RateCleanupInterval: 5 * time.Minute,

			ApiAdEnabled: true,
			ApiAdTextZh:  "推荐使用可靠的VPN服务保护您的隐私",
			ApiAdUrlZh:   "https://example.com/zh/vpn",
			ApiAdTextEn:  "Recommended VPN service for privacy",
			ApiAdUrlEn:   "https://example.com/en/vpn",

			WebAdEnabled: true,
			WebAdTextZh:  "推荐使用可靠的VPN服务保护您的隐私",
			WebAdUrlZh:   "https://example.com/zh/vpn",
			WebAdTextEn:  "Recommended VPN service for privacy",
			WebAdUrlEn:   "https://example.com/en/vpn",

			LogLevel:       "info",
			LogFileMaxSize: 50,
			LogFileMaxAge:  30,
			LogFileBackups: 7,
			LogIpMasking:   true,

			CorsEnabled:    true,
			JsonApiEnabled: true,
			AllApiEnabled:  false,

			ShutdownTimeout:   15 * time.Second,
			MaxHeaderBytes:    1024,
			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxConnsPerIP:     8,

			CfCidrPath:           "/etc/ip-lookup/cf-cidrs.txt",
			CfCidrReloadInterval: 5 * time.Minute,
			CfOnly:               false,

			GeoipEnabled:   false,
			GeoipDbPath:    "/var/lib/ip-lookup/GeoLite2-City.mmdb",
			GeoipAsnDbPath: "/var/lib/ip-lookup/GeoLite2-ASN.mmdb",

			MetricsListenAddr: "127.0.0.1:20013",

			Monitoring: MonitoringConfig{
				Enabled:                   false,
				CheckInterval:             60 * time.Second,
				AlertCooldown:             10 * time.Minute,
				ErrorRateThreshold:        0.05,
				P99LatencyThresholdMs:     2000,
				RateLimitHitRateThreshold: 0.10,
			},
		},
	}
}

const maxConfigSize = 1 << 20

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.configPath = path

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}
	if len(data) > maxConfigSize {
		return nil, fmt.Errorf("config file too large: %d bytes (max %d)", len(data), maxConfigSize)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	overrideFromEnv(cfg)
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func (cfg *Config) validate() error {
	for _, u := range []string{cfg.ApiAdUrlZh, cfg.ApiAdUrlEn, cfg.WebAdUrlZh, cfg.WebAdUrlEn} {
		if !validateAdURL(u) {
			return fmt.Errorf("invalid ad URL (must be http or https): %q", u)
		}
	}
	switch cfg.RateMode {
	case "both", "per_ip", "global":
	default:
		return fmt.Errorf("invalid rate_mode %q (must be both|per_ip|global)", cfg.RateMode)
	}
	for i, wc := range cfg.Monitoring.WebhookConfigs {
		if wc.URL == "" {
			return fmt.Errorf("monitoring.webhook_configs[%d].url must not be empty", i)
		}
		parsed, err := url.Parse(wc.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("monitoring.webhook_configs[%d].url must be a valid http(s) URL, got %q", i, wc.URL)
		}
		if wc.HTTPConfig != nil && wc.HTTPConfig.Authorization != nil {
			t := wc.HTTPConfig.Authorization.Type
			if t == "" {
				wc.HTTPConfig.Authorization.Type = "Bearer"
			} else if t != "Bearer" {
				return fmt.Errorf("monitoring.webhook_configs[%d].http_config.authorization.type must be \"Bearer\", got %q", i, t)
			}
		}
	}
	return nil
}

func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("LISTEN_ADDR_V4"); v != "" {
		cfg.ListenAddrV4 = v
	}
	if v := os.Getenv("LISTEN_ADDR_V6"); v != "" {
		cfg.ListenAddrV6 = v
	}
	if v := os.Getenv("PORT_V4"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.PortV4 = p
		}
	}
	if v := os.Getenv("PORT_V6"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.PortV6 = p
		}
	}
	if v := os.Getenv("RATE_PER_IP"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.RatePerIP = p
		}
	}
	if v := os.Getenv("RATE_PER_IP_BURST"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.RatePerIPBurst = p
		}
	}
	if v := os.Getenv("RATE_GLOBAL"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.RateGlobal = p
		}
	}
	if v := os.Getenv("RATE_GLOBAL_BURST"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.RateGlobalBurst = p
		}
	}
	if v := os.Getenv("RATE_ENABLED"); v != "" {
		cfg.RateEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("RATE_MODE"); v != "" {
		cfg.RateMode = v
	}

	if v := os.Getenv("API_AD_ENABLED"); v != "" {
		cfg.ApiAdEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("API_AD_TEXT_ZH"); v != "" {
		cfg.ApiAdTextZh = v
	}
	if v := os.Getenv("API_AD_URL_ZH"); v != "" {
		cfg.ApiAdUrlZh = v
	}
	if v := os.Getenv("API_AD_TEXT_EN"); v != "" {
		cfg.ApiAdTextEn = v
	}
	if v := os.Getenv("API_AD_URL_EN"); v != "" {
		cfg.ApiAdUrlEn = v
	}

	if v := os.Getenv("WEB_AD_ENABLED"); v != "" {
		cfg.WebAdEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("WEB_AD_TEXT_ZH"); v != "" {
		cfg.WebAdTextZh = v
	}
	if v := os.Getenv("WEB_AD_URL_ZH"); v != "" {
		cfg.WebAdUrlZh = v
	}
	if v := os.Getenv("WEB_AD_TEXT_EN"); v != "" {
		cfg.WebAdTextEn = v
	}
	if v := os.Getenv("WEB_AD_URL_EN"); v != "" {
		cfg.WebAdUrlEn = v
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("LOG_FILE_MAX_SIZE"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.LogFileMaxSize = p
		}
	}
	if v := os.Getenv("LOG_FILE_MAX_AGE"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.LogFileMaxAge = p
		}
	}
	if v := os.Getenv("LOG_FILE_BACKUPS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.LogFileBackups = p
		}
	}
	if v := os.Getenv("LOG_IP_MASKING"); v != "" {
		cfg.LogIpMasking = v == "true" || v == "1"
	}
	if v := os.Getenv("CORS_ENABLED"); v != "" {
		cfg.CorsEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("JSON_API_ENABLED"); v != "" {
		cfg.JsonApiEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("ALL_API_ENABLED"); v != "" {
		cfg.AllApiEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("TRUSTED_PROXY_CIDRS"); v != "" {
		cfg.TrustedProxyCIDRs = v
	}
	if v := os.Getenv("IP_DENYLIST"); v != "" {
		cfg.IPDenylist = v
	}
	if v := os.Getenv("UA_DENYLIST"); v != "" {
		cfg.UADenylist = v
	}
	if v := os.Getenv("CF_CIDR_PATH"); v != "" {
		cfg.CfCidrPath = v
	}
	if v := os.Getenv("CF_CIDR_RELOAD_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CfCidrReloadInterval = d
		}
	}
	if v := os.Getenv("CF_ONLY"); v != "" {
		cfg.CfOnly = v == "true" || v == "1"
	}
	if v := os.Getenv("GEOIP_ENABLED"); v != "" {
		cfg.GeoipEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("GEOIP_DB_PATH"); v != "" {
		cfg.GeoipDbPath = v
	}
	if v := os.Getenv("GEOIP_ASN_DB_PATH"); v != "" {
		cfg.GeoipAsnDbPath = v
	}
	if v := os.Getenv("METRICS_LISTEN_ADDR"); v != "" {
		cfg.MetricsListenAddr = v
	}
	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.ShutdownTimeout = time.Duration(p) * time.Second
		}
	}
	if v := os.Getenv("MONITORING_ENABLED"); v != "" {
		cfg.Monitoring.Enabled = v == "true" || v == "1"
	}
}

func (cfg *Config) ListenV4() string {
	return fmt.Sprintf("%s:%d", cfg.ListenAddrV4, cfg.PortV4)
}

func (cfg *Config) ListenV6() string {
	return fmt.Sprintf("[%s]:%d", cfg.ListenAddrV6, cfg.PortV6)
}

func (cfg *Config) ListenMetrics() string {
	return cfg.MetricsListenAddr
}

func (cfg *Config) StartHotReload(onReload func(*Config)) error {
	cfg.onReload = onReload
	var err error
	cfg.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify watcher: %w", err)
	}
	if err = cfg.watcher.Add(cfg.configPath); err != nil {
		return fmt.Errorf("watch config: %w", err)
	}
	go cfg.watchLoop()
	return nil
}

func (cfg *Config) watchLoop() {
	for {
		select {
		case event, ok := <-cfg.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Rename) != 0 {
				time.Sleep(200 * time.Millisecond)
				cfg.reload()
			}
		case err, ok := <-cfg.watcher.Errors:
			if !ok {
				return
			}
			logError(nil, "config watcher error", "error", err.Error())
		}
	}
}

func (cfg *Config) reload() {
	data, err := os.ReadFile(cfg.configPath)
	if err != nil {
		logError(nil, "config reload read error", "error", err.Error())
		return
	}
	if len(data) > maxConfigSize {
		logError(nil, "config reload file too large", "size", len(data), "max", maxConfigSize)
		return
	}
	newCfg := DefaultConfig()
	if err := yaml.Unmarshal(data, newCfg); err != nil {
		logError(nil, "config reload parse error", "error", err.Error())
		return
	}
	overrideFromEnv(newCfg)
	if err := newCfg.validate(); err != nil {
		logError(nil, "config reload validation failed, keeping current config", "error", err.Error())
		return
	}
	cfg.mu.Lock()
	cfg.ConfigValues = newCfg.ConfigValues
	onReload := cfg.onReload
	cfg.mu.Unlock()

	logInfo(nil, "config hot-reloaded")
	if onReload != nil {
		onReload(cfg)
	}
}

func (cfg *Config) detectLanguage(r *http.Request) string {
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang == "" {
		return "en"
	}
	langs := strings.Split(acceptLang, ",")
	if len(langs) == 0 {
		return "en"
	}
	primary := strings.TrimSpace(langs[0])
	primary = strings.Split(primary, ";")[0]
	if isSimplifiedZh(primary) {
		return "zh"
	}
	return "en"
}

// isSimplifiedZh reports whether the given language tag denotes Simplified
// Chinese. Only zh-CN / zh-Hans / zh-SG (and bare zh) are treated as zh;
// Traditional variants (zh-TW, zh-HK, zh-Hant, ...) fall back to en, matching
// the site-wide i18n policy.
func isSimplifiedZh(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "zh", "zh-cn", "zh-hans", "zh-sg":
		return true
	}
	return false
}

type adConfig struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

func (cfg *Config) GetApiAdText(r *http.Request) (string, string) {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	if !cfg.ApiAdEnabled {
		return "", ""
	}
	lang := cfg.detectLanguage(r)
	if lang == "zh" {
		return cfg.ApiAdTextZh, cfg.ApiAdUrlZh
	}
	return cfg.ApiAdTextEn, cfg.ApiAdUrlEn
}

func (cfg *Config) GetWebAdConfig(r *http.Request) *adConfig {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	if !cfg.WebAdEnabled {
		return nil
	}
	lang := cfg.detectLanguage(r)
	if lang == "zh" {
		return &adConfig{Text: cfg.WebAdTextZh, URL: cfg.WebAdUrlZh}
	}
	return &adConfig{Text: cfg.WebAdTextEn, URL: cfg.WebAdUrlEn}
}

func (cfg *Config) GetTrustedProxyCIDRs() []string {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return parseCommaList(cfg.TrustedProxyCIDRs)
}

func (cfg *Config) GetDenylistIPs() []string {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return parseCommaList(cfg.IPDenylist)
}

func (cfg *Config) GetDenylistUAs() []string {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return parseCommaList(cfg.UADenylist)
}

func (cfg *Config) GetRateEnabled() bool {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.RateEnabled
}

func (cfg *Config) GetRateMode() string {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.RateMode
}

func (cfg *Config) GetCfOnly() bool {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.CfOnly
}

func (cfg *Config) GetAllApiEnabled() bool {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.AllApiEnabled
}

func (cfg *Config) GetGeoipConfig() (bool, string, string) {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.GeoipEnabled, cfg.GeoipDbPath, cfg.GeoipAsnDbPath
}

func validateAdURL(u string) bool {
	if u == "" {
		return true
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func parseCommaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
