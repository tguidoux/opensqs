package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tguidoux/opensqs/apps/go/server/handlers"
	"github.com/tguidoux/opensqs/apps/go/server/health"
	"github.com/tguidoux/opensqs/apps/go/server/metrics"
	"github.com/tguidoux/opensqs/apps/go/server/middleware"
	tlsconfig "github.com/tguidoux/opensqs/apps/go/server/tls"
	"github.com/tguidoux/opensqs/apps/go/server/ui"
	"github.com/tguidoux/opensqs/pkgs/v1/config"
	env "github.com/tguidoux/opensqs/pkgs/v1/environment"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/badger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/sqlite"
)

func main() {
	// Load configuration
	configPath := os.Getenv(config.DefaultConfigEnvVar)
	if configPath == "" {
		// Try local file first, then container path
		if _, err := os.Stat(config.DefaultConfigPath); err == nil {
			configPath = config.DefaultConfigPath
		} else {
			configPath = "/apps/go/server/config.yaml"
			os.Setenv(config.DefaultConfigEnvVar, configPath)
		}
	}
	cfg := config.NewConfigFromEnv[ServerConfig]().Config

	// Initialize logger
	log := logger.New("opensqs-server", logger.UncontextualLoggerType)

	log.Infof("starting OpenSQS server", map[string]interface{}{
		"host":         cfg.Server.Host,
		"port":         cfg.Server.Port,
		"nodeAddress":  cfg.SQS.NodeAddress,
		"accountId":    cfg.SQS.AccountID,
		"region":       cfg.SQS.Region,
		"storageType":  cfg.SQS.StorageType,
		"strictLimits": cfg.SQS.StrictLimits,
		"environment":  cfg.Environment,
	})

	// Create queue manager
	limitsMode := queue.StrictMode
	if !cfg.SQS.StrictLimits {
		limitsMode = queue.RelaxedMode
	}
	limits := queue.NewLimits(limitsMode)

	// Create store factory (memory by default, SQLite or BadgerDB when configured)
	var storeFactory store.StoreFactory

	switch {
	case cfg.SQS.StorageType == "sqlite" && cfg.SQS.SQLitePath != "":
		db, err := sql.Open("sqlite", cfg.SQS.SQLitePath)
		if err != nil {
			log.Fatalf("failed to open SQLite database: %v", err)
		}
		// Enable WAL mode for better concurrent read performance
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			log.Fatalf("failed to set SQLite WAL mode: %v", err)
		}
		storeFactory = func(queueName string, visibilityTimeout int, serverSecret []byte, sc store.StoreConfig) store.Store {
			s, err := sqlite.NewSQLiteStore(db, queueName, visibilityTimeout, serverSecret, sc)
			if err != nil {
				log.Fatalf("failed to create SQLite store for queue %q: %v", queueName, err)
			}
			return s
		}
		defer db.Close()

	case cfg.SQS.StorageType == "badger" && cfg.SQS.BadgerPath != "":
		db, err := badger.Open(cfg.SQS.BadgerPath)
		if err != nil {
			log.Fatalf("failed to open BadgerDB database: %v", err)
		}
		storeFactory = func(queueName string, visibilityTimeout int, serverSecret []byte, sc store.StoreConfig) store.Store {
			s, err := badger.NewBadgerStore(db, queueName, visibilityTimeout, serverSecret, sc)
			if err != nil {
				log.Fatalf("failed to create BadgerDB store for queue %q: %v", queueName, err)
			}
			return s
		}
		defer db.Close()

	default:
		storeFactory = func(queueName string, visibilityTimeout int, serverSecret []byte, sc store.StoreConfig) store.Store {
			return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, sc)
		}
	}

	manager := queue.NewQueueManager(
		cfg.SQS.NodeAddress,
		cfg.SQS.AccountID,
		cfg.SQS.Region,
		[]byte(cfg.SQS.ServerSecret),
		storeFactory,
	)

	// Create startup queues from config
	for _, sq := range cfg.Queues.Startup {
		attrs := startupAttrsToQueueAttrs(sq.Attributes)
		_, err := manager.CreateQueue(sq.Name, attrs)
		if err != nil {
			log.Errorf("failed to create startup queue %q: %v", sq.Name, err)
		} else {
			log.Infof("created startup queue: %s", sq.Name)
		}
	}

	// Create the metrics collector (if metrics enabled)
	var metricsCollector *metrics.Collector
	if cfg.Metrics.Enabled {
		metricsCollector = metrics.NewCollector()
	}

	// Create the action handler
	handler := handlers.NewHandler(manager, limits, cfg.Queues.AutoCreate, metricsCollector)

	// Create the HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleSQSRequest(w, r, handler, log)
	})

	// Build middleware chain (order matters: outermost first)
	var middlewares []middleware.Middleware
	if cfg.RequestLogging.Enabled {
		middlewares = append(middlewares, middleware.RequestLogger(log))
	}
	if cfg.RateLimit.Enabled {
		if cfg.RateLimit.PerQueue {
			middlewares = append(middlewares, middleware.PerQueueRateLimiter(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst))
		} else {
			middlewares = append(middlewares, middleware.GlobalRateLimiter(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst))
		}
	}

	var sqsHandler http.Handler = mux
	if len(middlewares) > 0 {
		sqsHandler = middleware.Chain(middlewares...)(mux)
	}

	// Load TLS config for the SQS API server
	sqsTLS, err := tlsconfig.LoadFromConfig(cfg.Server.TLS.ToTLSConfig())
	if err != nil {
		log.Fatalf("failed to load SQS server TLS config: %v", err)
	}

	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      sqsHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	if sqsTLS != nil {
		httpServer.TLSConfig = sqsTLS
	}

	// Start health check server (only for non-local environments)
	if cfg.Environment != env.LOCAL {
		healthPort := cfg.Health.Port
		if healthPort == 0 {
			healthPort = 8001
		}
		healthTLS, err := tlsconfig.LoadFromConfig(cfg.Health.TLS.ToTLSConfig())
		if err != nil {
			log.Fatalf("failed to load health server TLS config: %v", err)
		}
		healthServer := health.NewServer(healthPort, healthTLS)
		if healthTLS != nil {
			healthServer.SetCertFiles(cfg.Health.TLS.CertFile, cfg.Health.TLS.KeyFile)
		}
		go func() {
			log.Infof("starting health check server on :%d", healthPort)
			if err := healthServer.Start(); err != nil && err != http.ErrServerClosed {
				log.Errorf("health check server error: %v", err)
			}
		}()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = healthServer.Stop(ctx)
		}()
	}

	// Start UI server (if enabled)
	var uiServer *ui.Server
	if cfg.UI.Enabled {
		uiTLS, err := tlsconfig.LoadFromConfig(cfg.UI.TLS.ToTLSConfig())
		if err != nil {
			log.Fatalf("failed to load UI server TLS config: %v", err)
		}
		uiServer = ui.NewServer(cfg.UI.Port, manager, log, cfg.Metrics.Enabled, uiTLS)
		if uiTLS != nil {
			uiServer.SetCertFiles(cfg.UI.TLS.CertFile, cfg.UI.TLS.KeyFile)
		}
		go func() {
			log.Infof("starting UI server on :%d", cfg.UI.Port)
			if err := uiServer.Start(); err != nil && err != http.ErrServerClosed {
				log.Errorf("UI server error: %v", err)
			}
		}()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = uiServer.Stop(ctx)
		}()
	}

	// Start metrics server (if enabled)
	var metricsServer *metrics.Server
	if cfg.Metrics.Enabled {
		metricsPort := cfg.Metrics.Port
		if metricsPort == 0 {
			metricsPort = 9326
		}
		metricsTLS, err := tlsconfig.LoadFromConfig(cfg.Metrics.TLS.ToTLSConfig())
		if err != nil {
			log.Fatalf("failed to load metrics server TLS config: %v", err)
		}
		metricsServer = metrics.NewServer(metricsPort, metricsTLS)
		if metricsTLS != nil {
			metricsServer.SetCertFiles(cfg.Metrics.TLS.CertFile, cfg.Metrics.TLS.KeyFile)
		}
		go func() {
			log.Infof("starting metrics server on :%d", metricsPort)
			if err := metricsServer.Start(); err != nil && err != http.ErrServerClosed {
				log.Errorf("metrics server error: %v", err)
			}
		}()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metricsServer.Stop(ctx)
		}()
	}

	// Start HTTP server in goroutine
	go func() {
		log.Infof("starting SQS server on %s:%d", cfg.Server.Host, cfg.Server.Port)
		var err error
		if sqsTLS != nil {
			err = httpServer.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		} else {
			err = httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Errorf("server forced to shutdown: %v", err)
	}

	// Shutdown all queue stores (close databases, release resources)
	if err := manager.Shutdown(ctx); err != nil {
		log.Errorf("queue manager shutdown error: %v", err)
	}

	log.Info("server stopped")
}
