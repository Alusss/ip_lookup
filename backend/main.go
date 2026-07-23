package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/time/rate"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	ready      atomic.Bool
	configPath string
	version    = "dev"
)

func main() {
	flag.StringVar(&configPath, "config", "/etc/ip-lookup/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	setupLogger(cfg)

	logInfo(nil, "starting ip-lookup", "version", version, "config", configPath)
	logInfo(nil, "configuration loaded",
		"listen_v4", cfg.ListenV4(),
		"listen_v6", cfg.ListenV6(),
		"listen_metrics", cfg.ListenMetrics(),
		"rate_enabled", cfg.RateEnabled,
		"rate_mode", cfg.RateMode,
		"rate_per_ip", cfg.RatePerIP,
		"rate_global", cfg.RateGlobal,
		"cf_only", cfg.CfOnly,
		"all_api_enabled", cfg.AllApiEnabled,
		"api_ad_enabled", cfg.ApiAdEnabled,
		"web_ad_enabled", cfg.WebAdEnabled,
		"json_api_enabled", cfg.JsonApiEnabled,
		"geoip_enabled", cfg.GeoipEnabled,
		"geoip_asn_db_path", cfg.GeoipAsnDbPath,
		"log_level", cfg.LogLevel,
	)

	extractor := NewIPExtractor(cfg.CfCidrPath, cfg.CfCidrReloadInterval)
	extractor.UpdateProxyCIDRs(cfg.GetTrustedProxyCIDRs())

	perIPLimiter := NewPerIPRateLimiter(rate.Limit(cfg.RatePerIP)/60, cfg.RatePerIPBurst, cfg.RateCleanupInterval)
	defer perIPLimiter.Stop()

	globalLimiter := NewGlobalRateLimiter(rate.Limit(cfg.RateGlobal), cfg.RateGlobalBurst)

	metrics := NewMetrics()
	connCounter := newConnCounter()

	geo := NewGeoIP(cfg.GeoipDbPath, cfg.GeoipAsnDbPath, cfg.GeoipEnabled)
	if geo != nil {
		defer geo.Close()
	}

	monitor := NewMonitor(cfg, metrics)
	monitor.Start()
	defer monitor.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/readyz", readyzHandler)
	mux.HandleFunc("/ad-config", adConfigHandler(cfg))
	mux.HandleFunc("/all", allHandler(cfg, extractor, perIPLimiter, globalLimiter, metrics, geo))
	mux.HandleFunc("/", rootHandler(cfg, extractor, perIPLimiter, globalLimiter, metrics, geo))

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metricsHandler(metrics))

	var handler http.Handler = mux
	handler = requestIDMiddleware(handler)
	handler = realIPMiddleware(handler, extractor)
	handler = recoveryMiddleware(handler)
	handler = securityHeadersMiddleware(handler)
	handler = metricsMiddleware(handler, metrics)
	handler = methodCheckMiddleware(handler)
	handler = bodyRejectionMiddleware(handler)
	handler = denylistMiddleware(handler, cfg, extractor)
	handler = connLimitMiddleware(handler, connCounter, cfg.MaxConnsPerIP, extractor)
	handler = corsMiddleware(handler, cfg.CorsEnabled)
	handler = loggingMiddleware(handler, cfg, extractor, metrics)

	srvV4 := &http.Server{
		Addr:              cfg.ListenV4(),
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	srvV6 := &http.Server{
		Addr:              cfg.ListenV6(),
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	srvMetrics := &http.Server{
		Addr:              cfg.ListenMetrics(),
		Handler:           metricsMux,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	errCh := make(chan error, 3)
	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		logInfo(nil, "listening v4", "addr", srvV4.Addr)
		if err := srvV4.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("v4: %w", err)
		}
	}()
	go func() {
		defer wg.Done()
		logInfo(nil, "listening v6", "addr", srvV6.Addr)
		if err := srvV6.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("v6: %w", err)
		}
	}()
	go func() {
		defer wg.Done()
		logInfo(nil, "listening metrics", "addr", srvMetrics.Addr)
		if err := srvMetrics.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("metrics: %w", err)
		}
	}()

	if err := cfg.StartHotReload(func(newCfg *Config) {
		extractor.UpdateProxyCIDRs(newCfg.GetTrustedProxyCIDRs())
		if geo != nil {
			en, city, asn := newCfg.GetGeoipConfig()
			geo.Configure(en, city, asn)
		}
	}); err != nil {
		logWarn(nil, "config hot reload not available", "error", err.Error())
	}

	ready.Store(true)

	select {
	case err := <-errCh:
		logError(nil, "server error", "error", err.Error())
		stop()
	case <-ctx.Done():
		logInfo(nil, "shutting down gracefully")
	}

	ready.Store(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	shutdownStart := time.Now()

	var shutdownErr error
	if err := srvV4.Shutdown(shutdownCtx); err != nil {
		shutdownErr = err
	}
	if err := srvV6.Shutdown(shutdownCtx); err != nil && shutdownErr == nil {
		shutdownErr = err
	}
	if err := srvMetrics.Shutdown(shutdownCtx); err != nil && shutdownErr == nil {
		shutdownErr = err
	}

	wg.Wait()
	shutdownDuration := time.Since(shutdownStart)
	metrics.SetShutdownDuration(int64(shutdownDuration.Seconds()))

	if shutdownErr != nil {
		logError(nil, "shutdown error", "error", shutdownErr.Error())
	}
	logInfo(nil, "shutdown complete", "duration_ms", shutdownDuration.Milliseconds())
}

func setupLogger(cfg *Config) {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logDir := "/var/log/ip-lookup"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		logWarn(nil, "failed to create log dir, using stdout only", "dir", logDir, "error", err.Error())
	}

	writer := &lumberjack.Logger{
		Filename:   fmt.Sprintf("%s/ip-lookup.log", logDir),
		MaxSize:    cfg.LogFileMaxSize,
		MaxBackups: cfg.LogFileBackups,
		MaxAge:     cfg.LogFileMaxAge,
		Compress:   true,
	}

	multiWriter := io.MultiWriter(os.Stdout, writer)
	handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

func logInfo(r *http.Request, msg string, args ...interface{}) {
	attrs := slogToAttrs(args...)
	if r != nil {
		attrs = append(attrs, slog.String("path", r.URL.Path))
		if id, ok := r.Context().Value(requestIDKey).(string); ok {
			attrs = append(attrs, slog.String("request_id", id))
		}
	}
	slog.LogAttrs(nil, slog.LevelInfo, msg, attrs...)
}

func logWarn(r *http.Request, msg string, args ...interface{}) {
	attrs := slogToAttrs(args...)
	if r != nil {
		if id, ok := r.Context().Value(requestIDKey).(string); ok {
			attrs = append(attrs, slog.String("request_id", id))
		}
	}
	slog.LogAttrs(nil, slog.LevelWarn, msg, attrs...)
}

func logError(r *http.Request, msg string, args ...interface{}) {
	attrs := slogToAttrs(args...)
	if r != nil {
		if id, ok := r.Context().Value(requestIDKey).(string); ok {
			attrs = append(attrs, slog.String("request_id", id))
		}
	}
	slog.LogAttrs(nil, slog.LevelError, msg, attrs...)
}

func slogToAttrs(args ...interface{}) []slog.Attr {
	if len(args) == 0 {
		return nil
	}
	attrs := make([]slog.Attr, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key := ""
		if k, ok := args[i].(string); ok {
			key = k
		} else {
			key = fmt.Sprintf("%v", args[i])
		}
		var val interface{}
		if i+1 < len(args) {
			val = args[i+1]
		}
		attrs = append(attrs, slog.Any(key, val))
	}
	return attrs
}
