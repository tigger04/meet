// ABOUTME: meet entrypoint. Loads config, serves the 8x8 JaaS meeting page
// ABOUTME: with a branded banner. Room name derived from the URL path.

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tigger04/meet/internal/server"
	"gopkg.in/yaml.v3"
)

// Version is the build identifier, overridden at link time.
var Version = "dev"

type appConfig struct {
	Addr    string `yaml:"addr"`
	BaseURL string `yaml:"base_url"`
	VpaasID string `yaml:"vpaas_id"`
}

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg := loadConfig(logger)

	if cfg.VpaasID == "" || cfg.VpaasID == "CHANGE_ME" {
		logger.Error("vpaas_id not configured — set it in your config YAML")
		os.Exit(1)
	}

	addr := resolveListenAddr(cfg)
	srv := server.New(server.Config{
		Addr:    addr,
		BaseURL: cfg.BaseURL,
		VpaasID: cfg.VpaasID,
		Logger:  logger,
	})

	logger.Info("meet starting", "version", Version, "addr", addr, "base_url", cfg.BaseURL)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		logger.Info("shutdown signal received", "signal", sig.String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown error", "error", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}

func loadConfig(logger *slog.Logger) appConfig {
	cfg := appConfig{
		Addr:    "127.0.0.1:18082",
		BaseURL: "https://meet.lobb.ie",
	}

	if data, err := os.ReadFile("config/defaults.yaml"); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			logger.Warn("config: failed to parse defaults.yaml", "error", err)
		}
	}

	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				logger.Warn("config: failed to parse host config", "path", configPath, "error", err)
			}
		}
	}

	return cfg
}

func resolveListenAddr(cfg appConfig) string {
	if a := os.Getenv("ADDR"); a != "" {
		return a
	}
	return cfg.Addr
}
