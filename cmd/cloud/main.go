package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/yingshulu/famfun/internal/cloud"
)

func main() {
	cfg := parseFlags()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cache := cloud.NewLRUCache(cfg.cacheSize)
	homeManager := cloud.NewHomeManager()
	streamProxy := cloud.NewStreamProxy(homeManager, cache)

	videoStore, err := cloud.NewVideoStore(cfg.dbPath)
	if err != nil {
		log.Fatalf("open video store: %v", err)
	}
	defer videoStore.Close()

	if cfg.adminPassword != "" {
		if _, err := videoStore.GetUserByUsername("admin"); err != nil {
			if _, err := videoStore.CreateUser("admin", cfg.adminPassword, "admin"); err != nil {
				log.Fatalf("create admin user: %v", err)
			}
			log.Println("admin user created")
		}
	}

	tlsConfig, err := loadTLSConfig(cfg.tlsCert, cfg.tlsKey)
	if err != nil {
		log.Fatalf("load TLS config: %v", err)
	}

	quicServer := cloud.NewQUICServer(cfg.quicAddr, tlsConfig, homeManager)
	go startQUICServer(ctx, quicServer)

	httpServer := cloud.NewCloudServer(homeManager, streamProxy, cache, videoStore, cfg.distDir)

	log.Printf("cloud server starting on %s (QUIC: %s)", cfg.httpAddr, cfg.quicAddr)

	if err := httpServer.Run(cfg.httpAddr); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

type config struct {
	httpAddr      string
	quicAddr      string
	cacheSize     int64
	distDir       string
	dbPath        string
	adminPassword string
	tlsCert       string
	tlsKey        string
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.httpAddr, "http-addr", ":8080", "HTTP listen address")
	flag.StringVar(&cfg.quicAddr, "quic-addr", ":4433", "QUIC listen address")
	flag.Int64Var(&cfg.cacheSize, "cache-size", 100*1024*1024, "LRU cache size in bytes")
	flag.StringVar(&cfg.distDir, "dist-dir", "./dist", "frontend dist directory")
	flag.StringVar(&cfg.dbPath, "db-path", "./famfun.db", "SQLite database path")
	flag.StringVar(&cfg.adminPassword, "admin-password", "", "create admin user with this password")
	flag.StringVar(&cfg.tlsCert, "tls-cert", "certs/server.crt", "TLS certificate file")
	flag.StringVar(&cfg.tlsKey, "tls-key", "certs/server.key", "TLS key file")
	flag.Parse()
	return cfg
}

func loadTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"famfun"},
	}, nil
}

func startQUICServer(ctx context.Context, server *cloud.QUICServer) {
	if err := server.Start(ctx); err != nil {
		log.Printf("QUIC server error: %v", err)
	}
}
