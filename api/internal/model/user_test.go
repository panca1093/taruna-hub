package model

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

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

func TestDefaultAvatarKey(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	male, female := GenderMale, GenderFemale
	teen, grown := time.Date(2008, 4, 17, 0, 0, 0, 0, time.UTC), time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	all := slices.Concat(avatarsMaleYoung, avatarsMaleAdult, avatarsFemaleYoung, avatarsFemaleAdult)
	if sorted := slices.Sorted(slices.Values(all)); len(slices.Compact(sorted)) != 35 || sorted[0] != "a01" || sorted[34] != "a35" {
		t.Fatalf("the four groups hold %d distinct keys, want each of a01..a35 once", len(slices.Compact(sorted)))
	}

	for _, tc := range []struct {
		name      string
		gender    *string
		birthDate *time.Time
		pool      []string
	}{
		{"teenage boy", &male, &teen, avatarsMaleYoung},
		{"grown man", &male, &grown, avatarsMaleAdult},
		{"teenage girl", &female, &teen, avatarsFemaleYoung},
		{"grown woman", &female, &grown, avatarsFemaleAdult},
		{"gender only", &female, nil, slices.Concat(avatarsFemaleYoung, avatarsFemaleAdult)},
		{"age only", nil, &teen, slices.Concat(avatarsMaleYoung, avatarsFemaleYoung)},
		{"nothing known", nil, nil, all},
	} {
		seen := map[string]bool{}
		for range 300 {
			id := uuid.New()
			key := DefaultAvatarKey(id, tc.gender, tc.birthDate, now)
			if !slices.Contains(tc.pool, key) {
				t.Fatalf("%s: got %s, want one of %v", tc.name, key, tc.pool)
			}
			if again := DefaultAvatarKey(id, tc.gender, tc.birthDate, now); again != key {
				t.Fatalf("%s: one user got %s then %s", tc.name, key, again)
			}
			seen[key] = true
		}
		if len(seen) != len(tc.pool) {
			t.Errorf("%s: 300 users used %d of %d avatars, want all", tc.name, len(seen), len(tc.pool))
		}
	}
}

func TestAgeAt(t *testing.T) {
	born := time.Date(2001, 11, 10, 0, 0, 0, 0, time.UTC)
	for day, want := range map[int]int{9: 24, 10: 25} {
		if got := AgeAt(born, time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC)); got != want {
			t.Errorf("age on 2026-11-%02d = %d, want %d", day, got, want)
		}
	}
}
