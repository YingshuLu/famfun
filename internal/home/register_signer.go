package home

import (
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/yingshulu/famfun/internal/protocol"
	pb "github.com/yingshulu/famfun/pkg/proto"
)

type RegisterSigner interface {
	SignRegisterRequest(req *pb.RegisterRequest) error
}

type RSASigner struct {
	privateKey *rsa.PrivateKey
}

func NewRSASignerFromFile(path string) (*RSASigner, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	key, err := protocol.ParseRSAPrivateKeyPEM(data)
	if err != nil {
		return nil, err
	}

	return &RSASigner{privateKey: key}, nil
}

func (s *RSASigner) SignRegisterRequest(req *pb.RegisterRequest) error {
	return protocol.SignRegisterRequest(req, s.privateKey)
}
