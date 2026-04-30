# Home Video Album - Distributed Video Streaming System

A YouTube-like home video streaming application with a cloud server and home server architecture. Videos are stored on a home server (private network), converted to HLS format, and streamed through a public cloud server.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    INTERNET / PUBLIC CLOUD                   │
│                    (Cloud Server :8080)                      │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  REST API: /api/videos, /api/stream/:id/:resource   │   │
│  │  Quic Connection: Persistent channel to home      │   │
│  │  Frontend: React SPA served on /                     │   │
│  └──────────────────────────────────────────────────────┘   │
└────────────────────────────┬────────────────────────────────┘
                             │
                        QUIC   Streams
                    (Persistent Connection)
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                  HOME NETWORK / PRIVATE                      │
│               (Home Server :9080 streaming)                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Video Scanner: Scans MP4s on startup               │   │
│  │  HLS Conversion: MP4 → HLS via ffmpeg               │   │
│  │  Thumbnail Generation: PNG thumbnails               │   │
│  │  Control Handler: Responds to playlist/segment      │   │
│  │  QUIC Connection: To cloud server                   │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
│  Local Storage:                                              │
│  ├─ ./videos/           (Original MP4 files)                │
│  ├─ ./video_streams/    (HLS .m3u8 + .ts files)            │
│  └─ ./thumbnails/       (PNG thumbnail cache)              │
└─────────────────────────────────────────────────────────────┘
```

## Components

### 1. Cloud Server (Go + Gin)
- **Port**: 8080
- **Features**:
  - REST API for video listings and metadata
  - QUIC server for persistent home server connections
  - HTTP endpoints for HLS playlists and segments (proxies to home server)
  - **Caching system**: LRU cache (100MB) for HLS segments to support multi-user playback
  - React frontend static file serving
  - Performance monitoring via `/api/cache-stats`
- **Multi-user Support**: ✅ Caches hot segments, supports 100+ concurrent users on 10Mbps home connection

### 2. Home Server (Go + Gin)
- **Port**: 9080 (for local streaming, not exposed publicly)
- **Features**:
  - MP4 etc format video files scanner and indexer (runs on startup)
  - ffmpeg-based HLS conversion (MP4 etc → .m3u8 + .ts segments)
  - Thumbnail generation (PNG, 320px width)
  - QUIC client connecting to cloud channel
  - Responds to playlist/segment requests from cloud

### 3. Frontend (React + TypeScript + Video.js)
- **Language**: TypeScript
- **Framework**: React 18
- **Video Player**: Video.js with HLS support
- **Build**: Webpack
- **Features**:
  - Video grid display with thumbnails
  - Video.js player with adaptive HLS streaming
  - Dynamic video list from cloud API
- **Key Files**: `frontend/src/App.tsx`, `frontend/src/components/`

## Protocol

### Control Channel Messages (WebSocket: Cloud → Home)

**Home Registration**:
```json
{
  "type": "register",
    "home_server_id": "uuid",
    "name": "Home Video Server"
}
```

**Heartbeat**:
Increase update the new added video to Cloud Server
```json
{
  "type": "heartbeat",
    "videos": [
      {
        "id": "video_id",
        "filename": "video.mp4",
        "title": "Video Title",
        "duration": 3600,
        "filesize": 1073741824,
        "thumbnail": "base64_encoded_png",
        "resolution": "1920x1080",
        "created_at": "2026-04-14T10:00:00Z",
        "home_server_id": "home_id"
      }
    ]
}
```

**Video List Update**:
```json
{
  "type": "video_list",
    "videos": [
      {
        "id": "video_id",
        "filename": "video.mp4",
        "title": "Video Title",
        "duration": 3600,
        "filesize": 1073741824,
        "thumbnail": "base64_encoded_png",
        "resolution": "1920x1080",
        "created_at": "2026-04-14T10:00:00Z",
        "home_server_id": "home_id"
      }
    ]
}
```

**Playlist Request**:
```json
{
  "type": "get_playlist",
    "video_id": "video_id"
}
```

**Segment Request**:
```json
{
  "type": "get_segment",
    "video_id": "video_id",
    "segment_name": "segment-0.ts"
}
```

## Setup & Installation

### Requirements
- Go 1.23+
- Node.js 18+
- ffmpeg and ffprobe
- macOS/Linux (or adjust paths for Windows)


This builds everything automatically. If npm fails, see [NPM_TROUBLESHOOTING.md](NPM_TROUBLESHOOTING.md)


## Data Flow

1. **Startup**:
   - Home server scans MP4 etc format videos
   - ffmpeg converts MP4 etc formats → HLS (.m3u8 + .ts files)
   - ffmpeg extracts thumbnails
   - Home server connects to cloud via persistent WebSocket
   - Home server sends video metadata to cloud

2. **User browses videos**:
   - Frontend loads, requests `/api/videos` from cloud
   - Cloud returns all videos from all connected home servers
   - Frontend displays video grid with thumbnails

3. **User clicks to play video**:
   - Frontend requests HLS playlist from `/api/stream/:homeID/:videoID/index.m3u8`
   - Cloud receives request, asks home server for playlist via control WebSocket
   - Home server responds with playlist content (updates segment URLs to cloud paths)
   - Cloud proxies playlist back to frontend
   - Video.js player requests segments
   - Each segment request goes to cloud, proxied to home server
   - Home server sends segment data back through cloud to client
   - Video.js plays the adaptive HLS stream


## Configuration

### Cloud Server
- Port: 8080 (edit `main.go` line with `router.Run()`)
- Frontend dist path: `./dist/` (relative to cloud binary)

### Home Server
- Cloud URL: `ws://localhost:8080/api/ws/home`
- Video directory: `./videos/`
- Stream output: `./video_streams/`
- Thumbnail output: `./thumbnails/`
- ffmpeg HLS segment duration: 6 seconds (edit `convertToHLS()`)

### Frontend
- API base URL: `/` (assumes same-domain, edit `App.tsx` axios calls if needed)
- Dev server port: 3000 (webpack config)
- Production port: Served by cloud server on 8080



For complete multi-user support details, see [MULTI_USER_GUIDE.md](MULTI_USER_GUIDE.md) and [CONCURRENCY_ANALYSIS.md](CONCURRENCY_ANALYSIS.md)

## Development

Built with:
- Go 1.23+ with Gin web framework
- React 18 + TypeScript
- Video.js for HLS playback
- ffmpeg for video processing
- Webpack for frontend bundling

## Code Rules
- Cloud / Home / Frontend are all under a go project
- Protocol / Data Models uses protobuf and are for common use
- Must use interface to code construction
- Must split into independent struct object based on reposibility, like: home server, Quic Connector, Video Scanner ...