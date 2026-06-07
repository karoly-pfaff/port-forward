package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/api"
	"portier/service/sources/logger"
	"portier/service/sources/manager"
	"portier/service/sources/options"
	"portier/service/sources/platform"
	"portier/service/sources/static"
	"portier/service/sources/version"
)

const windowsServiceName = "Portier"

func main() {
	log := logger.New()
	cwd, err := os.Getwd()
	if err != nil {
		log.Error("Failed to determine working directory", "error", err)
		os.Exit(1)
	}

	opts, err := options.Resolve(os.Args[1:], options.FromOSEnv(), cwd)
	if err != nil {
		log.Error("Failed to resolve server options", "error", err)
		os.Exit(2)
	}

	if platform.IsWindowsService() {
		err := platform.RunAsWindowsService(windowsServiceName, func(ctx context.Context) error {
			return runPortier(ctx, opts, log)
		})
		if err != nil {
			log.Error("Windows service failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// Console mode: graceful shutdown on Ctrl+C / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runPortier(ctx, opts, log); err != nil {
		log.Error("Server stopped with error", "error", err)
		os.Exit(1)
	}
}

func runPortier(ctx context.Context, opts options.Options, log *slog.Logger) error {
	startedAt := time.Now()
	staticAvailable := static.HasClient(opts.StaticDir)
	logStartup(log, opts, staticAvailable)

	if opts.Host == "0.0.0.0" {
		log.Warn("Management UI/API is listening on 0.0.0.0; this exposes Portier on the LAN")
	}
	if !staticAvailable {
		log.Warn("Static web UI is unavailable; API endpoints will still run", "staticDir", opts.StaticDir)
	}

	forwardManager, err := manager.NewFromConfig(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("Failed to load config: %w", err)
	}
	forwardManager.SetEventLogger(func(message string, args ...any) {
		log.Info(message, args...)
	})
	activityStore := &activity.Store{}
	forwardManager.SetActivityStore(activityStore)
	startedCount, err := forwardManager.StartEnabled()
	if err != nil {
		return fmt.Errorf("Failed to start enabled forwarding rules: %w", err)
	}
	log.Info("Loaded forwarding rules", "count", len(forwardManager.ListRules()), "started", startedCount)

	server := &http.Server{
		Addr: net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port)),
		Handler: api.NewHandler(api.Options{
			StaticDir:      opts.StaticDir,
			Manager:        forwardManager,
			StartedAt:      startedAt,
			ServiceOptions: opts,
			Version:        version.Version,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("Portier service listening", "address", server.Addr)
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			serverErrors <- nil
			return
		}
		serverErrors <- err
	}()

	select {
	case <-ctx.Done():
		log.Info("Shutdown signal received")
	case err := <-serverErrors:
		if err != nil {
			return fmt.Errorf("Server stopped unexpectedly: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("Graceful shutdown failed: %w", err)
	}
	forwardManager.StopAll()

	if err := <-serverErrors; err != nil {
		return fmt.Errorf("Server stopped with error: %w", err)
	}
	log.Info("Portier service stopped")
	return nil
}

func logStartup(log *slog.Logger, opts options.Options, staticAvailable bool) {
	log.Info("Starting Portier server",
		"host", opts.Host,
		"port", opts.Port,
		"configPath", opts.ConfigPath,
		"staticDir", opts.StaticDir,
		"staticDirSource", opts.StaticDirSource,
		"staticAvailable", staticAvailable,
		"service", opts.Service,
	)
}
