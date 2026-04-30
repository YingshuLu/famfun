# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

FamFun is a distributed home video streaming system. Videos are stored on a private **home server** and streamed through a public **cloud server** using QUIC protocol and HLS format. The cloud server exposes a REST API (Gin) and serves a React frontend; the home server processes videos with ffmpeg and connects to the cloud over QUIC.

## Build & Development Commands

```bash
make all              # Build everything (protobuf, frontend, cloud, home binaries)
make cloud            # Build cloud binary → bin/cloud
make home             # Build home binary → bin/home
make proto            # Regenerate protobuf Go code from proto/famfun.proto
make frontend         # Build React frontend (npm install + build in frontend/)
make test             # Run all Go tests: go test ./...
make clean            # Remove bin/, dist/, frontend artifacts
make dev-cert         # Generate self-signed TLS cert in certs/

# Run servers locally
make run-cloud        # ./bin/cloud --http-addr :8080 --quic-addr :4433 --dist-dir ./dist
make run-home         # ./bin/home --cloud-addr localhost:4433 --video-dir ./videos --tls-insecure

# Run a single test
go test ./internal/cloud/ -run TestCachePutAndGet -v
```

## Architecture

Two independent binaries communicate via Protobuf-over-QUIC:

```
Browser ──HTTP/HLS──► Cloud Server ──QUIC──► Home Server ──ffmpeg──► Video Files
                      (public)               (private/NAT)
```

**Cloud server** (`cmd/cloud/main.go`): Gin HTTP API + QUIC listener. Receives home server connections, proxies streaming requests, caches HLS segments (LRU), manages users (SQLite + bcrypt + JWT), and serves the React frontend.

**Home server** (`cmd/home/main.go`): QUIC client that connects to cloud, scans a video directory, converts videos to HLS via ffmpeg, generates thumbnails, and responds to streaming/metadata requests from cloud. Reconnects automatically with heartbeats every 30s.

### Key packages

- `internal/cloud/` — Cloud components: HTTP server (server.go), QUIC listener (quic_server.go), home registry (home_manager.go), stream proxy (stream_proxy.go), LRU cache (cache.go), user/video SQLite stores (userdb.go, videodb.go), JWT auth (auth.go)
- `internal/home/` — Home components: orchestrator (server.go), QUIC client (quic_client.go), video scanner (scanner.go), HLS converter (converter.go), thumbnail generator (thumbnail.go)
- `internal/model/` — Shared data types (Video struct with proto conversion)
- `internal/protocol/` — Message framing: 4-byte length header + Protobuf envelope, 50MB max
- `pkg/proto/` — Generated protobuf code from `proto/famfun.proto`

### Communication protocol

All cloud↔home messages use a Protobuf `Envelope` wrapper (defined in `proto/famfun.proto`). The home server maintains a persistent QUIC control stream for registration/heartbeats and opens data streams on-demand for playlist/segment/thumbnail requests.

### Role-based access

Three roles: `admin` (sees all videos, manages users), `member` (sees member+guest videos, can comment), `guest`/unauthenticated (guest-visibility videos only).

## Dependencies

Go 1.25.0 · Gin (HTTP) · quic-go (QUIC) · golang-jwt (JWT) · modernc.org/sqlite (pure-Go SQLite) · google/protobuf · ffmpeg/ffprobe (external, required by home server)
