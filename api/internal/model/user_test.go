package model

import "testing"

func TestValidAvatarKey(t *testing.T) {
	for _, key := range []string{"a01", "a09", "a10", "a16", "a29", "a35"} {
		if !ValidAvatarKey(key) {
			t.Errorf("ValidAvatarKey(%q) = false, want true", key)
		}
	}
	for _, key := range []string{"", "a00", "a36", "a40", "b01", "a1", "a001", "A01", "a0x"} {
		if ValidAvatarKey(key) {
			t.Errorf("ValidAvatarKey(%q) = true, want false", key)
		}
	}
}
