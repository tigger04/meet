// ABOUTME: meet entrypoint. Subcommands: serve (default) starts the web server,
// ABOUTME: token generates a moderator JWT URL for a given room.

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

	"github.com/golang-jwt/jwt/v5"
	"github.com/tigger04/meet/internal/server"
	"gopkg.in/yaml.v3"
)

// Version is the build identifier, overridden at link time.
var Version = "dev"

type appConfig struct {
	Addr        string  `yaml:"addr"`
	BaseURL     string  `yaml:"base_url"`
	DefaultRoom string  `yaml:"default_room"`
	Keys8x8     keys8x8 `yaml:"8x8-keys"`
}

type keys8x8 struct {
	AppID      string `yaml:"app-id"`
	KeyID      string `yaml:"key-id"`
	PrivateKey string `yaml:"private-key"`
	PublicKey  string `yaml:"public-key"`
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "token":
			runToken(os.Args[2:])
			return
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		case "--version", "-version":
			fmt.Println(Version)
			os.Exit(0)
		}
	}
	runServe(os.Args[1:])
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: meet <command> [options]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  serve   Start the web server (default)")
	fmt.Fprintln(os.Stderr, "  token   Generate a moderator JWT URL for a room")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	fmt.Fprintln(os.Stderr, "  -h, --help      Show this help")
	fmt.Fprintln(os.Stderr, "  --version       Print version and exit")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Run 'meet <command> -h' for command-specific help.")
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	versionFlag := fs.Bool("version", false, "print version and exit")
	configFlag := fs.String("config", "config/defaults.yaml", "comma-separated config files, merged left-to-right")
	fs.Parse(args)

	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg := loadConfig(*configFlag, logger)

	if cfg.Keys8x8.AppID == "" {
		logger.Error("app-id not configured — add it to a secrets YAML file")
		os.Exit(1)
	}

	srv := server.New(server.Config{
		Addr:        cfg.Addr,
		BaseURL:     cfg.BaseURL,
		AppID:       cfg.Keys8x8.AppID,
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

func runToken(args []string) {
	fs := flag.NewFlagSet("token", flag.ExitOnError)
	configFlag := fs.String("config", "config/defaults.yaml", "comma-separated config files, merged left-to-right")
	roomFlag := fs.String("room", "", "room name (required)")
	nameFlag := fs.String("name", "Moderator", "display name in the meeting")
	expiryFlag := fs.Duration("expiry", 2*time.Hour, "token validity duration")
	fs.Parse(args)

	if *roomFlag == "" {
		fmt.Fprintln(os.Stderr, "Usage: meet token --room <room-name> [--config ...] [--name ...] [--expiry ...]")
		os.Exit(2)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg := loadConfig(*configFlag, logger)

	if cfg.Keys8x8.AppID == "" {
		fmt.Fprintln(os.Stderr, "error: app-id not configured")
		os.Exit(1)
	}
	if cfg.Keys8x8.KeyID == "" {
		fmt.Fprintln(os.Stderr, "error: key-id not configured")
		os.Exit(1)
	}
	if cfg.Keys8x8.PrivateKey == "" {
		fmt.Fprintln(os.Stderr, "error: private-key not configured")
		os.Exit(1)
	}

	privKey := parsePrivateKey(cfg.Keys8x8.PrivateKey)

	now := time.Now()
	claims := jwt.MapClaims{
		"aud":  "jitsi",
		"iss":  "chat",
		"sub":  cfg.Keys8x8.AppID,
		"room": "*",
		"iat":  now.Unix(),
		"nbf":  now.Unix(),
		"exp":  now.Add(*expiryFlag).Unix(),
		"context": map[string]interface{}{
			"user": map[string]interface{}{
				"name":      *nameFlag,
				"moderator": "true",
			},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = cfg.Keys8x8.KeyID

	signed, err := token.SignedString(privKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to sign JWT: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s/%s?jwt=%s\n", cfg.BaseURL, *roomFlag, signed)
}

func parsePrivateKey(pemStr string) *rsa.PrivateKey {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		fmt.Fprintln(os.Stderr, "error: private-key: failed to decode PEM block")
		os.Exit(1)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: private-key: failed to parse: %v\n", err)
		os.Exit(1)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		fmt.Fprintln(os.Stderr, "error: private-key: not an RSA key")
		os.Exit(1)
	}
	return rsaKey
}

func loadConfig(paths string, logger *slog.Logger) appConfig {
	cfg := appConfig{
		Addr:        "127.0.0.1:18085",
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
