package cloud

import (
	"strings"
	"testing"
)

func TestGenerateAvatar(t *testing.T) {
	svg := generateAvatar("kamlu")
	if !strings.Contains(svg, "<svg") {
		t.Error("expected SVG output")
	}
	if !strings.Contains(svg, ">K</text>") {
		t.Errorf("expected initial 'K', got: %s", svg)
	}
}

func TestGenerateAvatarDeterministic(t *testing.T) {
	a := generateAvatar("alice")
	b := generateAvatar("alice")
	if a != b {
		t.Error("same username should produce same avatar")
	}
}

func TestGenerateAvatarDifferentUsers(t *testing.T) {
	a := generateAvatar("alice")
	b := generateAvatar("bob")
	if a == b {
		t.Error("different usernames should produce different avatars")
	}
}

func TestGenerateAvatarEmpty(t *testing.T) {
	svg := generateAvatar("")
	if !strings.Contains(svg, ">?</text>") {
		t.Errorf("expected '?' for empty username, got: %s", svg)
	}
}
