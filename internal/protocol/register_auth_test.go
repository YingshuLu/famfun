package protocol

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	pb "github.com/yingshulu/famfun/pkg/proto"
)

func TestRegisterRequestSignatureRoundTrip(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	req := &pb.RegisterRequest{
		HomeServerId: "home-1",
		Name:         "Living Room",
		Challenge:    []byte("nonce-1"),
	}
	if err := SignRegisterRequest(req, privateKey); err != nil {
		t.Fatalf("sign request: %v", err)
	}
	if len(req.Signature) == 0 {
		t.Fatal("expected signature to be set")
	}

	if err := VerifyRegisterRequest(req, &privateKey.PublicKey); err != nil {
		t.Fatalf("verify request: %v", err)
	}
}

func TestRegisterRequestSignatureRejectsTampering(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	req := &pb.RegisterRequest{
		HomeServerId: "home-1",
		Name:         "Living Room",
		Challenge:    []byte("nonce-1"),
	}
	if err := SignRegisterRequest(req, privateKey); err != nil {
		t.Fatalf("sign request: %v", err)
	}

	req.Name = "Garage"
	if err := VerifyRegisterRequest(req, &privateKey.PublicKey); err == nil {
		t.Fatal("expected verification to fail after tampering")
	}
}
