package cloud

import (
	"context"
	"crypto/tls"
	"encoding/hex"
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
	videoStore   *VideoStore
	listener     *quic.Listener
}

func NewQUICServer(addr string, tlsConfig *tls.Config, registry HomeRegistry, videoStore *VideoStore) *QUICServer {
	return &QUICServer{
		addr:         addr,
		tlsConfig:    tlsConfig,
		homeRegistry: registry,
		videoStore:   videoStore,
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

	log.Println("Quic server accept a new connection")
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
	challengeReqEnv, err := protocol.ReadMessage(stream)
	if err != nil {
		return nil, fmt.Errorf("read register challenge request: %w", err)
	}
	challengeReq := challengeReqEnv.GetRegisterChallenge()
	if challengeReq == nil || len(challengeReq.Challenge) == 0 {
		return nil, fmt.Errorf("expected register challenge request, got %T", challengeReqEnv.Payload)
	}

	serverChallenge, err := protocol.GenerateChallenge(16)
	if err != nil {
		return nil, fmt.Errorf("generate register challenge: %w", err)
	}

	challengeRspEnv := &pb.Envelope{
		Payload: &pb.Envelope_RegisterChallenge{
			RegisterChallenge: &pb.RegisterChallenge{
				Challenge: serverChallenge,
			},
		},
	}
	if err := protocol.WriteMessage(stream, challengeRspEnv); err != nil {
		return nil, fmt.Errorf("send register challenge: %w", err)
	}

	env, err := protocol.ReadMessage(stream)
	if err != nil {
		return nil, fmt.Errorf("read register message: %w", err)
	}

	req := env.GetRegisterRequest()
	if req == nil {
		return nil, fmt.Errorf("expected RegisterRequest, got %T", env.Payload)
	}

	combinedChallenge := append(challengeReq.Challenge, serverChallenge...)
	if err := s.verifyRegisterRequest(req, combinedChallenge); err != nil {
		return nil, fmt.Errorf("verify register request: %w", err)
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

func (s *QUICServer) verifyRegisterRequest(req *pb.RegisterRequest, expectedChallenge []byte) error {
	if s.videoStore == nil {
		return fmt.Errorf("video store not configured")
	}
	if len(expectedChallenge) == 0 {
		return fmt.Errorf("missing expected register challenge")
	}
	if len(req.Challenge) == 0 {
		return fmt.Errorf("missing register challenge in request")
	}
	if !equalBytes(req.Challenge, expectedChallenge) {
		return fmt.Errorf(
			"register challenge mismatch: expected %s, got %s",
			hex.EncodeToString(expectedChallenge),
			hex.EncodeToString(req.Challenge),
		)
	}

	storedKey, err := s.videoStore.GetHomePublicKey(req.HomeServerId)
	if err != nil {
		return err
	}

	pub, err := protocol.ParseRSAPublicKeyPEM([]byte(storedKey.PublicKey))
	if err != nil {
		return fmt.Errorf("parse public key for %s: %w", req.HomeServerId, err)
	}

	if err := protocol.VerifyRegisterRequest(req, pub); err != nil {
		return err
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
	var toAdd = make(map[string]struct{})
	var addedVideos []*model.Video
	for _, id := range videoIDs {
		if _, ok := s.homeRegistry.QueryVideo(hc.homeID, id); ok {
			continue
		}
		if _, exists := toAdd[id]; exists {
			continue
		}

		v, err := s.fetchVideoInfo(hc, id)
		if err != nil {
			log.Printf("fetch video info %s from %s: %v", id, hc.homeID, err)
			continue
		}
		addedVideos = append(addedVideos, v)
		toAdd[id] = struct{}{}
	}

	if len(toAdd) == 0 {
		return
	}

	s.homeRegistry.SyncHomeVideos(hc.homeID, addedVideos, nil)
	if len(toAdd) > 0 {
		log.Printf("added %d new videos from %s", len(toAdd), hc.homeID)
	}
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
