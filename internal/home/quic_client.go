package home

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/yingshulu/famfun/internal/protocol"
	pb "github.com/yingshulu/famfun/pkg/proto"
)

type MessageHandler interface {
	HandleGetPlaylist(req *pb.GetPlaylistRequest) *pb.GetPlaylistResponse
	HandleGetSegment(req *pb.GetSegmentRequest) *pb.GetSegmentResponse
	HandleGetVideoInfo(req *pb.GetVideoInfoRequest) *pb.GetVideoInfoResponse
	HandleGetThumbnail(req *pb.GetThumbnailRequest) *pb.GetThumbnailResponse
	HandleUpdateVideoInfo(req *pb.UpdateVideoInfoRequest) *pb.UpdateVideoInfoResponse
	HandleUploadVideo(stream io.ReadWriter, req *pb.UploadVideoRequest)
	HandleScan(req *pb.ScanRequest) *pb.ScanResponse
}

type CloudConnector interface {
	Connect(ctx context.Context, cloudAddr string) error
	SendEnvelope(env *pb.Envelope) error
	SetMessageHandler(handler MessageHandler)
	GetRegisterChallenge() ([]byte, error)
	Disconnected() <-chan struct{}
	Close() error
}

type QUICClient struct {
	tlsInsecure       bool
	conn              *quic.Conn
	controlStream     *quic.Stream
	handler           MessageHandler
	registerChallenge []byte
	mu                sync.Mutex
	closed            bool
	disconnected      chan struct{}
}

func NewQUICClient(tlsInsecure bool) *QUICClient {
	return &QUICClient{
		tlsInsecure: tlsInsecure,
	}
}

func (c *QUICClient) SetMessageHandler(handler MessageHandler) {
	c.handler = handler
}

func (c *QUICClient) Connect(ctx context.Context, cloudAddr string) error {
	tlsConfig := c.buildTLSConfig()

	conn, err := quic.DialAddr(ctx, cloudAddr, tlsConfig, &quic.Config{
		KeepAlivePeriod: 15 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("quic dial: %w", err)
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		conn.CloseWithError(0, "failed to open stream")
		return fmt.Errorf("open control stream: %w", err)
	}
	log.Println("Quic client connect with openning stream success")

	clientChallenge, err := protocol.GenerateChallenge(16)
	if err != nil {
		conn.CloseWithError(0, "failed to generate register challenge")
		return fmt.Errorf("generate register challenge: %w", err)
	}

	challengeReqEnv := &pb.Envelope{
		Payload: &pb.Envelope_RegisterChallenge{
			RegisterChallenge: &pb.RegisterChallenge{
				Challenge: clientChallenge,
			},
		},
	}
	if err := protocol.WriteMessage(stream, challengeReqEnv); err != nil {
		conn.CloseWithError(0, "failed to send register challenge")
		return fmt.Errorf("send register challenge: %w", err)
	}

	challengeRspEnv, err := protocol.ReadMessage(stream)
	if err != nil {
		conn.CloseWithError(0, "failed to read register challenge")
		return fmt.Errorf("read register challenge: %w", err)
	}
	log.Println("Quic client read register challenge success")

	serverChallenge := challengeRspEnv.GetRegisterChallenge()
	if serverChallenge == nil || len(serverChallenge.Challenge) == 0 {
		conn.CloseWithError(0, "missing register challenge")
		return fmt.Errorf("expected register challenge, got %T", challengeRspEnv.Payload)
	}
	log.Println("Quic client got register challenge")

	c.mu.Lock()
	c.conn = conn
	c.controlStream = stream
	c.registerChallenge = append(clientChallenge, serverChallenge.Challenge...)
	c.closed = false
	c.disconnected = make(chan struct{})
	c.mu.Unlock()

	go c.controlReadLoop()
	go c.acceptDataStreams(ctx)

	return nil
}

func (c *QUICClient) Disconnected() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disconnected
}

func (c *QUICClient) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	if c.controlStream != nil {
		c.controlStream.Close()
	}
	if c.conn != nil {
		return c.conn.CloseWithError(0, "shutdown")
	}
	return nil
}

func (c *QUICClient) SendEnvelope(env *pb.Envelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.controlStream == nil {
		return fmt.Errorf("not connected")
	}
	return protocol.WriteMessage(c.controlStream, env)
}

func (c *QUICClient) GetRegisterChallenge() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.registerChallenge) == 0 {
		return nil, fmt.Errorf("register challenge not available")
	}
	return append([]byte(nil), c.registerChallenge...), nil
}

func (c *QUICClient) buildTLSConfig() *tls.Config {
	return &tls.Config{
		NextProtos:         []string{"famfun"},
		InsecureSkipVerify: c.tlsInsecure,
	}
}

func (c *QUICClient) controlReadLoop() {
	defer func() {
		c.mu.Lock()
		if !c.closed && c.disconnected != nil {
			close(c.disconnected)
		}
		c.mu.Unlock()
	}()

	for {
		env, err := protocol.ReadMessage(c.controlStream)
		if err != nil {
			if !c.isClosed() {
				log.Printf("control read error: %v", err)
			}
			return
		}
		c.dispatchControlMessage(env)
	}
}

func (c *QUICClient) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *QUICClient) dispatchControlMessage(env *pb.Envelope) {
	switch p := env.Payload.(type) {
	case *pb.Envelope_RegisterResponse:
		c.handleRegisterResponse(p.RegisterResponse)
	case *pb.Envelope_HeartbeatResponse:
		c.handleHeartbeatResponse(p.HeartbeatResponse)
	default:
		log.Printf("unexpected control message: %T", env.Payload)
	}
}

func (c *QUICClient) handleRegisterResponse(resp *pb.RegisterResponse) {
	if resp.Success {
		log.Printf("registered with cloud: %s", resp.Message)
	} else {
		log.Printf("registration failed: %s", resp.Message)
	}
}

func (c *QUICClient) handleHeartbeatResponse(resp *pb.HeartbeatResponse) {
	if !resp.Success {
		log.Printf("heartbeat failed")
	}
}

func (c *QUICClient) acceptDataStreams(ctx context.Context) {
	for {
		stream, err := c.conn.AcceptStream(ctx)
		if err != nil {
			if !c.isClosed() {
				log.Printf("accept data stream error: %v", err)
			}
			return
		}
		go c.handleDataStream(stream)
	}
}

func (c *QUICClient) handleDataStream(stream *quic.Stream) {
	defer stream.Close()

	env, err := protocol.ReadMessage(stream)
	if err != nil {
		log.Printf("read data stream request error: %v", err)
		return
	}

	resp := c.dispatchDataRequest(stream, env)
	if resp == nil {
		return
	}

	if err := protocol.WriteMessage(stream, resp); err != nil {
		log.Printf("write data stream response error: %v", err)
	}
}

func (c *QUICClient) dispatchDataRequest(stream *quic.Stream, env *pb.Envelope) *pb.Envelope {
	if c.handler == nil {
		log.Printf("no handler set for data request")
		return nil
	}

	switch p := env.Payload.(type) {
	case *pb.Envelope_GetPlaylistRequest:
		resp := c.handler.HandleGetPlaylist(p.GetPlaylistRequest)
		return &pb.Envelope{
			Payload: &pb.Envelope_GetPlaylistResponse{
				GetPlaylistResponse: resp,
			},
		}
	case *pb.Envelope_GetSegmentRequest:
		resp := c.handler.HandleGetSegment(p.GetSegmentRequest)
		return &pb.Envelope{
			Payload: &pb.Envelope_GetSegmentResponse{
				GetSegmentResponse: resp,
			},
		}
	case *pb.Envelope_GetVideoInfoRequest:
		resp := c.handler.HandleGetVideoInfo(p.GetVideoInfoRequest)
		return &pb.Envelope{
			Payload: &pb.Envelope_GetVideoInfoResponse{
				GetVideoInfoResponse: resp,
			},
		}
	case *pb.Envelope_GetThumbnailRequest:
		resp := c.handler.HandleGetThumbnail(p.GetThumbnailRequest)
		return &pb.Envelope{
			Payload: &pb.Envelope_GetThumbnailResponse{
				GetThumbnailResponse: resp,
			},
		}
	case *pb.Envelope_UpdateVideoInfoRequest:
		resp := c.handler.HandleUpdateVideoInfo(p.UpdateVideoInfoRequest)
		return &pb.Envelope{
			Payload: &pb.Envelope_UpdateVideoInfoResponse{
				UpdateVideoInfoResponse: resp,
			},
		}
	case *pb.Envelope_UploadVideoRequest:
		c.handler.HandleUploadVideo(stream, p.UploadVideoRequest)
		return nil
	case *pb.Envelope_ScanRequest:
		resp := c.handler.HandleScan(p.ScanRequest)
		return &pb.Envelope{
			Payload: &pb.Envelope_ScanResponse{
				ScanResponse: resp,
			},
		}
	default:
		log.Printf("unexpected data stream message: %T", env.Payload)
		return nil
	}
}

func ConnectWithRetry(ctx context.Context, client CloudConnector, cloudAddr string) error {
	backoff := time.Second
	maxBackoff := 60 * time.Second

	for {
		err := client.Connect(ctx, cloudAddr)
		if err == nil {
			return nil
		}

		log.Printf("connect failed: %v, retrying in %v", err, backoff)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff = nextBackoff(backoff, maxBackoff)
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}
