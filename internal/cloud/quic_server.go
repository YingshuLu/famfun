package cloud

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"sync"

	"github.com/quic-go/quic-go"

	"github.com/yingshulu/famfun/internal/model"
	"github.com/yingshulu/famfun/internal/protocol"
	pb "github.com/yingshulu/famfun/pkg/proto"
)

type QUICServer struct {
	addr         string
	tlsConfig    *tls.Config
	homeRegistry HomeRegistry
	listener     *quic.Listener
}

func NewQUICServer(addr string, tlsConfig *tls.Config, registry HomeRegistry) *QUICServer {
	return &QUICServer{
		addr:         addr,
		tlsConfig:    tlsConfig,
		homeRegistry: registry,
	}
}

func (s *QUICServer) Start(ctx context.Context) error {
	listener, err := quic.ListenAddr(s.addr, s.tlsConfig, &quic.Config{
		KeepAlivePeriod: 15,
	})
	if err != nil {
		return fmt.Errorf("quic listen: %w", err)
	}
	s.listener = listener
	log.Printf("QUIC server listening on %s", s.addr)

	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("QUIC accept error: %v", err)
			continue
		}
		go s.handleConnection(ctx, conn)
	}
}

func (s *QUICServer) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *QUICServer) handleConnection(ctx context.Context, conn *quic.Conn) {
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		log.Printf("accept stream error: %v", err)
		return
	}

	homeConn, err := s.performRegistration(conn, stream)
	if err != nil {
		log.Printf("registration failed: %v", err)
		stream.Close()
		return
	}
	defer s.cleanupConnection(homeConn)

	log.Printf("home server registered: %s (%s)", homeConn.homeID, homeConn.homeName)
	s.controlLoop(stream, homeConn)
}

func (s *QUICServer) performRegistration(conn *quic.Conn, stream *quic.Stream) (*quicHomeConn, error) {
	env, err := protocol.ReadMessage(stream)
	if err != nil {
		return nil, fmt.Errorf("read register message: %w", err)
	}

	req := env.GetRegisterRequest()
	if req == nil {
		return nil, fmt.Errorf("expected RegisterRequest, got %T", env.Payload)
	}

	hc := newQuicHomeConn(req.HomeServerId, req.Name, conn, stream)

	if err := s.homeRegistry.RegisterHome(req.HomeServerId, req.Name, hc); err != nil {
		return nil, fmt.Errorf("register home: %w", err)
	}

	resp := &pb.Envelope{
		Payload: &pb.Envelope_RegisterResponse{
			RegisterResponse: &pb.RegisterResponse{
				Success: true,
				Message: "registered",
			},
		},
	}
	if err := protocol.WriteMessage(stream, resp); err != nil {
		s.homeRegistry.UnregisterHome(req.HomeServerId)
		return nil, fmt.Errorf("send register response: %w", err)
	}

	return hc, nil
}

func (s *QUICServer) cleanupConnection(hc *quicHomeConn) {
	s.homeRegistry.UnregisterHome(hc.homeID)
	log.Printf("home server disconnected: %s", hc.homeID)
}

func (s *QUICServer) controlLoop(stream *quic.Stream, hc *quicHomeConn) {
	for {
		env, err := protocol.ReadMessage(stream)
		if err != nil {
			log.Printf("control read error from %s: %v", hc.homeID, err)
			return
		}
		s.dispatchControlMessage(stream, hc, env)
	}
}

func (s *QUICServer) dispatchControlMessage(stream *quic.Stream, hc *quicHomeConn, env *pb.Envelope) {
	switch p := env.Payload.(type) {
	case *pb.Envelope_HeartbeatRequest:
		s.handleHeartbeat(stream, hc, p.HeartbeatRequest)
	case *pb.Envelope_VideoListUpdate:
		s.handleVideoListUpdate(hc, p.VideoListUpdate)
	default:
		log.Printf("unexpected control message from %s: %T", hc.homeID, env.Payload)
	}
}

func (s *QUICServer) handleHeartbeat(stream *quic.Stream, hc *quicHomeConn, req *pb.HeartbeatRequest) {
	s.syncVideos(hc, req.VideoIds)

	resp := &pb.Envelope{
		Payload: &pb.Envelope_HeartbeatResponse{
			HeartbeatResponse: &pb.HeartbeatResponse{Success: true},
		},
	}
	if err := protocol.WriteMessage(stream, resp); err != nil {
		log.Printf("send heartbeat response error: %v", err)
	}
}

func (s *QUICServer) handleVideoListUpdate(hc *quicHomeConn, update *pb.VideoListUpdate) {
	s.syncVideos(hc, update.VideoIds)
	log.Printf("video list updated from %s: %d videos", hc.homeID, len(update.VideoIds))
}

func (s *QUICServer) syncVideos(hc *quicHomeConn, videoIDs []string) {
	var videos []*model.Video
	for _, id := range videoIDs {
		if v, ok := s.homeRegistry.QueryVideo(hc.homeID, id); ok {
			videos = append(videos, v)
			continue
		}
		v, err := s.fetchVideoInfo(hc, id)
		if err != nil {
			log.Printf("fetch video info %s from %s: %v", id, hc.homeID, err)
			continue
		}
		videos = append(videos, v)
	}
	s.homeRegistry.UpdateVideos(hc.homeID, videos)
}

func (s *QUICServer) fetchVideoInfo(hc *quicHomeConn, videoID string) (*model.Video, error) {
	req := &pb.Envelope{
		Payload: &pb.Envelope_GetVideoInfoRequest{
			GetVideoInfoRequest: &pb.GetVideoInfoRequest{
				VideoId: videoID,
			},
		},
	}

	resp, err := sendDataRequest(hc, req)
	if err != nil {
		return nil, fmt.Errorf("request video info: %w", err)
	}

	vr := resp.GetGetVideoInfoResponse()
	if vr == nil {
		return nil, fmt.Errorf("unexpected response type: %T", resp.Payload)
	}
	if !vr.Success {
		return nil, fmt.Errorf("video info error: %s", vr.Error)
	}

	return model.ProtoToVideo(vr.Video), nil
}

type quicHomeConn struct {
	homeID        string
	homeName      string
	conn          *quic.Conn
	controlStream *quic.Stream
	mu            sync.Mutex
}

func newQuicHomeConn(homeID, homeName string, conn *quic.Conn, controlStream *quic.Stream) *quicHomeConn {
	return &quicHomeConn{
		homeID:        homeID,
		homeName:      homeName,
		conn:          conn,
		controlStream: controlStream,
	}
}

func (c *quicHomeConn) HomeID() string   { return c.homeID }
func (c *quicHomeConn) HomeName() string { return c.homeName }

func (c *quicHomeConn) Close() error {
	c.controlStream.Close()
	return c.conn.CloseWithError(0, "closing")
}

func (c *quicHomeConn) OpenDataStream() (*quic.Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx := context.Background()
	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("open data stream: %w", err)
	}
	return stream, nil
}
