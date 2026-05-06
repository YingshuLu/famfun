package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"

	"github.com/yingshulu/famfun/internal/home"
)

func main() {
	cfg := parseFlags()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ensureDirs(cfg)

	scanner := home.NewVideoScanner()
	converter := home.NewHLSConverter()
	thumbGen := home.NewThumbnailGenerator()
	client := home.NewQUICClient(cfg.tlsInsecure)

	server := home.NewHomeServer(
		cfg.homeID, cfg.homeName,
		cfg.videoDir, cfg.streamDir, cfg.thumbDir,
		scanner, converter, thumbGen, client,
	)
	if cfg.privateKeyPath != "" {
		signer, err := home.NewRSASignerFromFile(cfg.privateKeyPath)
		if err != nil {
			log.Fatalf("load RSA private key: %v", err)
		}
		server.SetRegisterSigner(signer)
	}

	log.Printf("home server starting (id=%s, name=%s)", cfg.homeID, cfg.homeName)

	if err := server.Run(ctx, cfg.cloudAddr); err != nil {
		log.Fatalf("home server error: %v", err)
	}
}

type config struct {
	cloudAddr      string
	videoDir       string
	streamDir      string
	thumbDir       string
	homeID         string
	homeName       string
	privateKeyPath string
	tlsInsecure    bool
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.cloudAddr, "cloud-addr", "localhost:4433", "cloud QUIC server address")
	flag.StringVar(&cfg.videoDir, "video-dir", "./videos", "video source directory")
	flag.StringVar(&cfg.streamDir, "stream-dir", "./video_streams", "HLS output directory")
	flag.StringVar(&cfg.thumbDir, "thumb-dir", "./thumbnails", "thumbnail output directory")
	flag.StringVar(&cfg.homeID, "home-id", "", "home server ID (auto-generated if empty)")
	flag.StringVar(&cfg.homeName, "home-name", "", "home server name (hostname if empty)")
	flag.StringVar(&cfg.privateKeyPath, "rsa-private-key", "", "RSA private key PEM used to sign register requests")
	flag.BoolVar(&cfg.tlsInsecure, "tls-insecure", false, "skip TLS verification")
	flag.Parse()

	if cfg.homeID == "" {
		cfg.homeID = uuid.New().String()
	}
	if cfg.homeName == "" {
		cfg.homeName = defaultHomeName()
	}

	return cfg
}

func defaultHomeName() string {
	name, err := os.Hostname()
	if err != nil {
		return "Home Server"
	}
	return name
}

func ensureDirs(cfg config) {
	for _, dir := range []string{cfg.videoDir, cfg.streamDir, cfg.thumbDir} {
		os.MkdirAll(dir, 0o755)
	}
}
