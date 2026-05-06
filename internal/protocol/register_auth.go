package protocol

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	pb "github.com/yingshulu/famfun/pkg/proto"
	"google.golang.org/protobuf/proto"
)

func GenerateChallenge(length int) ([]byte, error) {
	challenge := make([]byte, length)
	if _, err := rand.Read(challenge); err != nil {
		return nil, err
	}
	return challenge, nil
}

func registerRequestPayload(req *pb.RegisterRequest) ([]byte, error) {
	canonical := &pb.RegisterRequest{
		HomeServerId: req.HomeServerId,
		Name:         req.Name,
		Challenge:    req.Challenge,
	}
	return proto.MarshalOptions{Deterministic: true}.Marshal(canonical)
}

func SignRegisterRequest(req *pb.RegisterRequest, key *rsa.PrivateKey) error {
	payload, err := registerRequestPayload(req)
	if err != nil {
		return fmt.Errorf("marshal register request: %w", err)
	}

	digest := sha256.Sum256(payload)
	signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], nil)
	if err != nil {
		return fmt.Errorf("sign register request: %w", err)
	}

	req.Signature = signature
	return nil
}

func VerifyRegisterRequest(req *pb.RegisterRequest, key *rsa.PublicKey) error {
	if len(req.Signature) == 0 {
		return fmt.Errorf("missing register request signature")
	}

	payload, err := registerRequestPayload(req)
	if err != nil {
		return fmt.Errorf("marshal register request: %w", err)
	}

	digest := sha256.Sum256(payload)
	if err := rsa.VerifyPSS(key, crypto.SHA256, digest[:], req.Signature, nil); err != nil {
		return fmt.Errorf("verify register request: %w", err)
	}
	return nil
}

func ParseRSAPublicKeyPEM(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM data")
	}

	switch block.Type {
	case "PUBLIC KEY":
		pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse public key: %w", err)
		}
		pub, ok := pubAny.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key is not RSA")
		}
		return pub, nil
	case "RSA PUBLIC KEY":
		pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse RSA public key: %w", err)
		}
		return pub, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q", block.Type)
	}
}

func ParseRSAPrivateKeyPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM data")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse RSA private key: %w", err)
		}
		return key, nil
	case "PRIVATE KEY":
		keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		key, ok := keyAny.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q", block.Type)
	}
}
