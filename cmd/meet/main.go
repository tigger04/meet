// ABOUTME: meet entrypoint. Loads config, serves the 8x8 JaaS meeting page
// ABOUTME: with a branded banner. Room name derived from the URL path.

package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tigger04/meet/internal/server"
	"gopkg.in/yaml.v3"
)

// Version is the build identifier, overridden at link time.
var Version = "dev"

type appConfig struct {
	Addr        string    `yaml:"addr"`
	BaseURL     string    `yaml:"base_url"`
	DefaultRoom string    `yaml:"default_room"`
	Keys8x8     keys8x8   `yaml:"8x8-keys"`
}

type keys8x8 struct {
	AppID      string `yaml:"app-id"`
	KeyID      string `yaml:"key-id"`
	PrivateKey string `yaml:"private-key"`
	PublicKey  string `yaml:"public-key"`
}

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	configFlag := flag.String("config", "config/defaults.yaml", "comma-separated config files, merged left-to-right")
	flag.Parse()

	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg := loadConfig(*configFlag, logger)

	if cfg.Keys8x8.AppID == "" {
		logger.Error("app_id not configured — add it to a secrets YAML file")
		os.Exit(1)
	}

	srv := server.New(server.Config{
		Addr:    cfg.Addr,
		BaseURL: cfg.BaseURL,
		Keys8x8: server.Keys8x8{
			AppID:      cfg.Keys8x8.AppID,
			PrivateKey: cfg.Keys8x8.PrivateKey,
			PublicKey:  cfg.Keys8x8.PublicKey,
		},
		DefaultRoom: cfg.DefaultRoom,
		Logger:      logger,
	})

	logger.Info("meet starting", "version", Version, "addr", cfg.Addr, "base_url", cfg.BaseURL)

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

func loadConfig(paths string, logger *slog.Logger) appConfig {
	cfg := appConfig{
		Addr:        "127.0.0.1:18082",
		DefaultRoom: "lobby",
	}

	for _, p := range strings.Split(paths, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				logger.Info("config: file not found, skipping", "path", p)
				continue
			}
			logger.Error("config: read error", "path", p, "error", err)
			os.Exit(1)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			logger.Error("config: parse error", "path", p, "error", err)
			os.Exit(1)
		}
		logger.Info("config: loaded", "path", p)
	}

	return cfg
}
