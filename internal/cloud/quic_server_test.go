package cloud

import (
	"testing"
)

func TestQuicHomeConnFields(t *testing.T) {
	hc := &quicHomeConn{
		homeID:   "h1",
		homeName: "Home 1",
	}

	if hc.HomeID() != "h1" {
		t.Errorf("HomeID = %q, want %q", hc.HomeID(), "h1")
	}
	if hc.HomeName() != "Home 1" {
		t.Errorf("HomeName = %q, want %q", hc.HomeName(), "Home 1")
	}
}
