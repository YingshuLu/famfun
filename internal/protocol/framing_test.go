package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"

	pb "github.com/yingshulu/famfun/pkg/proto"
)

func TestMarshalUnmarshalEnvelope(t *testing.T) {
	env := &pb.Envelope{
		Payload: &pb.Envelope_RegisterRequest{
			RegisterRequest: &pb.RegisterRequest{
				HomeServerId: "home-1",
				Name:         "Test Home",
			},
		},
	}

	data, err := MarshalEnvelope(env)
	if err != nil {
		t.Fatalf("MarshalEnvelope: %v", err)
	}

	got, err := UnmarshalEnvelope(data)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope: %v", err)
	}

	req := got.GetRegisterRequest()
	if req == nil {
		t.Fatal("expected RegisterRequest payload")
	}
	if req.HomeServerId != "home-1" {
		t.Errorf("HomeServerId = %q, want %q", req.HomeServerId, "home-1")
	}
	if req.Name != "Test Home" {
		t.Errorf("Name = %q, want %q", req.Name, "Test Home")
	}
}

func TestWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteHeader(&buf, 42); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if buf.Len() != 4 {
		t.Fatalf("header length = %d, want 4", buf.Len())
	}
	got := binary.BigEndian.Uint32(buf.Bytes())
	if got != 42 {
		t.Errorf("header value = %d, want 42", got)
	}
}

func TestReadHeader(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(99))

	got, err := ReadHeader(&buf)
	if err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	if got != 99 {
		t.Errorf("got %d, want 99", got)
	}
}

func TestReadHeaderEOF(t *testing.T) {
	var buf bytes.Buffer
	_, err := ReadHeader(&buf)
	if err == nil {
		t.Fatal("expected error on empty reader")
	}
}

func TestValidateMessageSize(t *testing.T) {
	if err := ValidateMessageSize(1024); err != nil {
		t.Errorf("unexpected error for small message: %v", err)
	}
	if err := ValidateMessageSize(MaxMessageSize + 1); err == nil {
		t.Error("expected error for oversized message")
	}
}

func TestReadPayload(t *testing.T) {
	data := []byte("hello world")
	buf := bytes.NewReader(data)

	got, err := ReadPayload(buf, uint32(len(data)))
	if err != nil {
		t.Fatalf("ReadPayload: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("got %q, want %q", got, data)
	}
}

func TestWriteReadMessage(t *testing.T) {
	env := &pb.Envelope{
		Payload: &pb.Envelope_HeartbeatRequest{
			HeartbeatRequest: &pb.HeartbeatRequest{
				HomeServerId: "home-2",
				VideoIds:     []string{"vid-1", "vid-2"},
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, env); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	got, err := ReadMessage(&buf)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	hb := got.GetHeartbeatRequest()
	if hb == nil {
		t.Fatal("expected HeartbeatRequest payload")
	}
	if hb.HomeServerId != "home-2" {
		t.Errorf("HomeServerId = %q, want %q", hb.HomeServerId, "home-2")
	}
	if len(hb.VideoIds) != 2 {
		t.Fatalf("len(VideoIds) = %d, want 2", len(hb.VideoIds))
	}
	if hb.VideoIds[0] != "vid-1" {
		t.Errorf("VideoIds[0] = %q, want %q", hb.VideoIds[0], "vid-1")
	}
}

func TestWriteReadMessageMultiple(t *testing.T) {
	var buf bytes.Buffer

	envs := []*pb.Envelope{
		{Payload: &pb.Envelope_RegisterRequest{RegisterRequest: &pb.RegisterRequest{HomeServerId: "h1"}}},
		{Payload: &pb.Envelope_RegisterResponse{RegisterResponse: &pb.RegisterResponse{Success: true}}},
		{Payload: &pb.Envelope_GetPlaylistRequest{GetPlaylistRequest: &pb.GetPlaylistRequest{VideoId: "v1", RequestId: "r1"}}},
	}

	for _, env := range envs {
		if err := WriteMessage(&buf, env); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}

	for i, want := range envs {
		got, err := ReadMessage(&buf)
		if err != nil {
			t.Fatalf("ReadMessage[%d]: %v", i, err)
		}
		_ = got
		_ = want
	}
}

func TestReadMessageTooLarge(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(MaxMessageSize+1))

	_, err := ReadMessage(&buf)
	if err == nil {
		t.Fatal("expected error for oversized message")
	}
}
