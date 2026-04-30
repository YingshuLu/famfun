package protocol

import (
	"encoding/binary"
	"fmt"
	"io"

	pb "github.com/yingshulu/famfun/pkg/proto"
	"google.golang.org/protobuf/proto"
)

const MaxMessageSize = 50 * 1024 * 1024

func MarshalEnvelope(env *pb.Envelope) ([]byte, error) {
	return proto.Marshal(env)
}

func UnmarshalEnvelope(data []byte) (*pb.Envelope, error) {
	env := &pb.Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return nil, err
	}
	return env, nil
}

func WriteHeader(w io.Writer, length uint32) error {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)
	_, err := w.Write(header)
	return err
}

func ReadHeader(r io.Reader) (uint32, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(header), nil
}

func ValidateMessageSize(length uint32) error {
	if length > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", length, MaxMessageSize)
	}
	return nil
}

func ReadPayload(r io.Reader, length uint32) ([]byte, error) {
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

func WriteMessage(w io.Writer, env *pb.Envelope) error {
	data, err := MarshalEnvelope(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	if err := WriteHeader(w, uint32(len(data))); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

func ReadMessage(r io.Reader) (*pb.Envelope, error) {
	length, err := ReadHeader(r)
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if err := ValidateMessageSize(length); err != nil {
		return nil, err
	}
	data, err := ReadPayload(r, length)
	if err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}
	return UnmarshalEnvelope(data)
}
