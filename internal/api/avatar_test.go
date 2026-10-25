package api

import (
	"github.com/exzco/bbs/internal/models"
	"testing"
)

func TestLegacyAvatarURL(t *testing.T) {
	for _, tc := range []struct{ stored, want string }{
		{"abc.png", "/assets/avatars/abc.png"},
		{"/avatars/abc.png", "/avatars/abc.png"},
		{"/assets/avatars/abc.png", "/assets/avatars/abc.png"},
		{"https://example.org/a.png", "https://example.org/a.png"},
		{"javascript:alert(1)", "/avatar/test.svg"},
		{"../secret", "/avatar/test.svg"},
		{"", "/avatar/test.svg"},
	} {
		if got := avatarURL(&models.User{Username: "test", AvatarURL: tc.stored}); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.stored, got, tc.want)
		}
	}
}
