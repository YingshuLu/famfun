package home

import (
	"testing"
	"time"
)

func TestNextBackoff(t *testing.T) {
	tests := []struct {
		current time.Duration
		max     time.Duration
		want    time.Duration
	}{
		{1 * time.Second, 60 * time.Second, 2 * time.Second},
		{2 * time.Second, 60 * time.Second, 4 * time.Second},
		{30 * time.Second, 60 * time.Second, 60 * time.Second},
		{60 * time.Second, 60 * time.Second, 60 * time.Second},
	}

	for _, tt := range tests {
		got := nextBackoff(tt.current, tt.max)
		if got != tt.want {
			t.Errorf("nextBackoff(%v, %v) = %v, want %v", tt.current, tt.max, got, tt.want)
		}
	}
}

func TestQUICClientBuildTLSConfig(t *testing.T) {
	client := NewQUICClient(true)
	cfg := client.buildTLSConfig()

	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify = true")
	}
	if len(cfg.NextProtos) != 1 || cfg.NextProtos[0] != "famfun" {
		t.Errorf("NextProtos = %v, want [famfun]", cfg.NextProtos)
	}
}

func TestQUICClientBuildTLSConfigSecure(t *testing.T) {
	client := NewQUICClient(false)
	cfg := client.buildTLSConfig()

	if cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify = false")
	}
}

func TestSendEnvelopeNotConnected(t *testing.T) {
	client := NewQUICClient(false)
	err := client.SendEnvelope(nil)
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestIsClosedInitiallyFalse(t *testing.T) {
	client := NewQUICClient(false)
	if client.isClosed() {
		t.Error("expected not closed initially")
	}
}

func TestCloseMarksAsClosed(t *testing.T) {
	client := NewQUICClient(false)
	client.Close()
	if !client.isClosed() {
		t.Error("expected closed after Close()")
	}
}
