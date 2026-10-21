package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"uptime-mon/internal/alerting"
	"uptime-mon/internal/api"
	"uptime-mon/internal/config"
	"uptime-mon/internal/maintenance"
	"uptime-mon/internal/prober"
	"uptime-mon/internal/storage"
	"uptime-mon/web"
)

func main() {
	var (
		apiFlag        bool
		tokenFlag      bool
		newFlag        bool
		listFlag       bool
		revokeFlag     string
		nameFlag       string
		configFlag     bool
		validateFlag   bool
		configDirFlag  string
		dataDirFlag    string
		portFlag       int
		bindFlag       string
	)

	// Custom flag parsing to support flags like --api --token --new
	fs := flag.NewFlagSet("uptime-mon", flag.ContinueOnError)
	fs.BoolVar(&apiFlag, "api", false, "API management")
	fs.BoolVar(&tokenFlag, "token", false, "Token operations")
	fs.BoolVar(&newFlag, "new", false, "Generate new token")
	fs.BoolVar(&listFlag, "list", false, "List tokens")
	fs.StringVar(&revokeFlag, "revoke", "", "Revoke token by ID")
	fs.StringVar(&nameFlag, "name", "Homepage Widget", "Token label name")

	fs.BoolVar(&configFlag, "config", false, "Configuration actions")
	fs.BoolVar(&validateFlag, "validate", false, "Validate configuration files")

	fs.StringVar(&configDirFlag, "config-dir", "./config", "Path to config directory")
	fs.StringVar(&dataDirFlag, "data-dir", "./data", "Path to data directory")
	fs.IntVar(&portFlag, "port", 8080, "Server HTTP port")
	fs.StringVar(&bindFlag, "bind", "0.0.0.0", "Server bind interface")

	// Filter out subcommand "serve" if present as first positional argument
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	dbPath := filepath.Join(dataDirFlag, "uptime.db")

	// 1. Handle Config Validation: uptime-mon --config --validate
	if configFlag || validateFlag {
		runConfigValidate(configDirFlag)
		return
	}

	// 2. Handle Token Management: uptime-mon --api --token ...
	if apiFlag || tokenFlag {
		runTokenCLI(dbPath, newFlag, listFlag, revokeFlag, nameFlag, portFlag)
		return
	}

	// 3. Default: Run server daemon (uptime-mon serve or plain uptime-mon)
	runDaemon(configDirFlag, dbPath, bindFlag, portFlag)
}

func runConfigValidate(configDir string) {
	fmt.Printf("Validating configuration files in %s...\n", configDir)
	rc, err := config.Load(configDir)
	if err != nil {
		fmt.Printf("✗ Configuration load error: %v\n", err)
		os.Exit(1)
	}

	errs := config.Validate(rc)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Printf("✗ %s\n", e)
		}
		fmt.Printf("Configuration validation failed with %d error(s).\n", len(errs))
		os.Exit(1)
	}

	fmt.Printf("✓ Global config: OK\n")
	fmt.Printf("✓ %d zones defined: OK\n", len(rc.Zones))
	fmt.Printf("✓ %d endpoints defined: OK\n", len(rc.Endpoints))
	fmt.Printf("✓ %d one-off maintenance, %d scheduled: OK\n", len(rc.OneOffMaintenance), len(rc.ScheduledMaintenance))
	fmt.Printf("✓ %d webhooks configured: OK\n", len(rc.Alerting.Webhooks))
	fmt.Printf("Configuration valid. 0 errors found.\n")
}

func runTokenCLI(dbPath string, isNew, isList bool, revokeID, name string, port int) {
	db, err := storage.Open(dbPath)
	if err != nil {
		fmt.Printf("Error opening database at %s: %v\n", dbPath, err)
		os.Exit(1)
	}
	defer db.Close()

	if isNew {
		tokenID, tokenSecret, err := db.GenerateToken(name)
		if err != nil {
			fmt.Printf("Failed to generate token: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Generated API Token for Homepage Integration:")
		fmt.Println("---------------------------------------------------------------")
		fmt.Printf("Token ID:     %s\n", tokenID)
		fmt.Printf("Token Secret: %s\n", tokenSecret)
		fmt.Println()
		fmt.Println("Homepage services.yaml snippet:")
		fmt.Println("---------------------------------------------------------------")
		fmt.Println("- Uptime Monitor:")
		fmt.Println("    icon: uptime-kuma.png")
		fmt.Printf("    href: http://localhost:%d/\n", port)
		fmt.Println("    description: System Availability")
		fmt.Println("    widget:")
		fmt.Println("      type: customapi")
		fmt.Printf("      url: http://localhost:%d/api/v1/status/summary\n", port)
		fmt.Println("      refreshInterval: 10000")
		fmt.Println("      headers:")
		fmt.Printf("        X-API-Token-Id: %s\n", tokenID)
		fmt.Printf("        X-API-Token-Secret: %s\n", tokenSecret)
		fmt.Println("      mappings:")
		fmt.Println("        - field: \"Hosts Up\"")
		fmt.Println("          label: \"Up\"")
		fmt.Println("          format: number")
		fmt.Println("        - field: \"Hosts Down\"")
		fmt.Println("          label: \"Down\"")
		fmt.Println("          format: number")
		fmt.Println("        - field: \"Uptime\"")
		fmt.Println("          label: \"Uptime\"")
		fmt.Println("          format: percent")
		fmt.Println("---------------------------------------------------------------")
		return
	}

	if isList {
		tokens, err := db.ListTokens()
		if err != nil {
			fmt.Printf("Failed to list tokens: %v\n", err)
			os.Exit(1)
		}

		if len(tokens) == 0 {
			fmt.Println("No API tokens found. Generate one with: uptime-mon --api --token --new")
			return
		}

		fmt.Printf("%-20s %-20s %-22s %-22s %-10s\n", "TOKEN ID", "LABEL", "CREATED AT", "LAST USED AT", "STATUS")
		for _, t := range tokens {
			lastUsed := "Never"
			if t.LastUsedAt != nil {
				lastUsed = t.LastUsedAt.Format("2006-01-02 15:04:05")
			}
			status := "ACTIVE"
			if t.Revoked {
				status = "REVOKED"
			}
			fmt.Printf("%-20s %-20s %-22s %-22s %-10s\n",
				t.TokenID, t.Name, t.CreatedAt.Format("2006-01-02 15:04:05"), lastUsed, status)
		}
		return
	}

	if revokeID != "" {
		if err := db.RevokeToken(revokeID); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Token %s successfully revoked.\n", revokeID)
		return
	}

	fmt.Println("Usage: uptime-mon --api --token [--new [--name <label>]] | [--list] | [--revoke <token_id>]")
	os.Exit(1)
}

func runDaemon(configDir, dbPath, bind string, port int) {
	log.Printf("[daemon] starting uptime-mon v1.0.0...")

	// 1. Open SQLite database
	db, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("[fatal] database initialization failed: %v", err)
	}
	defer db.Close()

	// 2. Load configurations
	cfg, err := config.Load(configDir)
	if err != nil {
		log.Fatalf("[fatal] failed to load configuration from %s: %v", configDir, err)
	}
	if errs := config.Validate(cfg); len(errs) > 0 {
		log.Fatalf("[fatal] configuration validation failed: %v", errs)
	}

	// 3. Initialize alert dispatcher
	dispatcher := alerting.NewDispatcher(db, cfg.Alerting.Webhooks)

	// 4. Initialize maintenance manager
	maintMgr := maintenance.NewManager(cfg, dispatcher)
	stopMaint := make(chan struct{})
	go maintMgr.StartBackgroundChecker(stopMaint)

	// 5. Initialize probe scheduler
	scheduler := prober.NewScheduler(db, maintMgr)
	scheduler.OnEvent(dispatcher.HandleTransitionEvent)
	scheduler.UpdateConfig(cfg)

	// 6. Initialize SSE Broker and HTTP API Server
	broker := api.NewSSEBroker()
	broker.AttachProberEvents(scheduler)

	server := api.NewServer(cfg, db, broker, web.DistFS)
	server.IsMaintenanceActive = maintMgr.IsEndpointInMaintenance

	// 7. Watch ./config for hot-reloads
	watcherCtx, cancelWatcher := context.WithCancel(context.Background())
	watcher, err := config.NewWatcher(configDir, func(newCfg *config.RuntimeConfig) {
		maintMgr.UpdateConfig(newCfg)
		scheduler.UpdateConfig(newCfg)
		dispatcher.UpdateWebhooks(newCfg.Alerting.Webhooks)
		server.UpdateConfig(newCfg)
		broker.BroadcastEvent("summary", map[string]interface{}{"status": "config_reloaded"})
	})
	if err != nil {
		log.Printf("[warning] could not start fsnotify watcher: %v", err)
	} else {
		go watcher.Start(watcherCtx)
	}

	// 8. Start HTTP Server
	addr := fmt.Sprintf("%s:%d", bind, port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: server,
	}

	go func() {
		log.Printf("[daemon] listening on http://%s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[fatal] http server failed: %v", err)
		}
	}()

	// 9. Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Printf("[daemon] shutting down gracefully...")
	cancelWatcher()
	close(stopMaint)
	scheduler.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)

	log.Printf("[daemon] shutdown complete.")
}
