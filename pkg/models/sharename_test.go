// Copyright 2021 Synology Inc.

package models

import (
	"strings"
	"testing"
)

const testVolName = "pvc-729da2ea-f821-4b37-9e9e-7ec95c3c230b"

func TestShareUUIDSliceIsStableAndHex(t *testing.T) {
	got := ShareUUIDSlice(testVolName)
	if want := "729da2eaf"; got != want {
		t.Fatalf("ShareUUIDSlice = %q, want %q", got, want)
	}
	if strings.Contains(got, "-") {
		t.Fatalf("lookup key must be pure hex, got %q", got)
	}
	// Deriving it twice must give the same answer — it is the lookup key.
	if ShareUUIDSlice(testVolName) != got {
		t.Fatal("ShareUUIDSlice is not deterministic")
	}
}

func TestGenShareNameWithApp(t *testing.T) {
	cases := []struct {
		name string
		app  string
		want string
	}{
		{"short app name", "myapp", "k8s-csi-pvc-myapp-729da2eaf"},
		{"no app name", "", "k8s-csi-pvc-729da2eaf"},
		{"app name is truncated, uuid is not", "averyverylongappname", "k8s-csi-pvc-averyveryl-729da2eaf"},
		{"namespace-style name is sanitised", "My_App/Data", "k8s-csi-pvc-my-appdata-729da2eaf"},
		{"unusable app name falls back", "///", "k8s-csi-pvc-729da2eaf"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GenShareNameWithApp(testVolName, c.app)
			if got != c.want {
				t.Fatalf("GenShareNameWithApp(_, %q) = %q, want %q", c.app, got, c.want)
			}
			if len(got) > MaxShareLen {
				t.Fatalf("name %q exceeds MaxShareLen %d", got, MaxShareLen)
			}
			// The uuid must survive intact whatever the app name does.
			if !strings.HasSuffix(got, ShareUUIDSlice(testVolName)) {
				t.Fatalf("name %q lost its uuid lookup key", got)
			}
		})
	}
}

// Even a pathological app name must not push the share name past the DSM limit
// nor eat into the uuid.
func TestGenShareNameWithAppRespectsLimit(t *testing.T) {
	got := GenShareNameWithApp(testVolName, strings.Repeat("x", 200))
	if len(got) > MaxShareLen {
		t.Fatalf("name %q is %d chars, exceeds MaxShareLen %d", got, len(got), MaxShareLen)
	}
	if !strings.HasSuffix(got, "-"+ShareUUIDSlice(testVolName)) {
		t.Fatalf("uuid key missing from %q", got)
	}
}

// Lookup must resolve BOTH naming schemes, or every share provisioned before the
// naming change would be orphaned.
func TestShareNameMatchesVolume(t *testing.T) {
	legacy := GenShareName(testVolName) // k8s-csi-pvc-729da2ea-f821-4b37-9
	withApp := GenShareNameWithApp(testVolName, "myapp")
	noApp := GenShareNameWithApp(testVolName, "")

	for _, name := range []string{legacy, withApp, noApp} {
		if !ShareNameMatchesVolume(name, testVolName) {
			t.Fatalf("ShareNameMatchesVolume(%q, volName) = false, want true", name)
		}
	}

	// A different volume must never match.
	other := "pvc-88bf99c1-6561-4d71-b389-000000000000"
	for _, name := range []string{legacy, withApp, noApp} {
		if ShareNameMatchesVolume(name, other) {
			t.Fatalf("ShareNameMatchesVolume(%q, otherVol) = true, want false", name)
		}
	}
}

// An archived share keeps the same uuid key, so it is still identifiable as
// belonging to its original volume.
func TestArchivedShareKeepsLookupKey(t *testing.T) {
	active := GenShareNameWithApp(testVolName, "myapp")
	archived := GenArchivedShareName(active)

	if !strings.HasPrefix(archived, ShareStatusArchived) {
		t.Fatalf("archived name %q lost its status prefix", archived)
	}
	if !strings.HasSuffix(archived, "-"+ShareUUIDSlice(testVolName)) {
		t.Fatalf("archived name %q lost its uuid key", archived)
	}
	if len(archived) != len(active) {
		t.Fatalf("archiving changed length: %q -> %q", active, archived)
	}
}

// Regression: "managed by the driver" must never be mistaken for "archived".
// The purge path relies on this distinction; conflating them would delete a live
// volume instead of archiving it.
func TestIsArchivedShareName(t *testing.T) {
	cases := []struct {
		share string
		want  bool
	}{
		{"del-csi-pvc-myapp-729da2eaf", true},
		{"del-csi-pvc-729da2eaf", true},
		{"k8s-csi-pvc-myapp-729da2eaf", false}, // active — must NOT look archived
		{"k8s-csi-pvc-729da2eaf", false},
		{"photo", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsArchivedShareName(c.share); got != c.want {
			t.Fatalf("IsArchivedShareName(%q) = %v, want %v", c.share, got, c.want)
		}
	}

	// Every active name must archive into something that reports as archived.
	active := GenShareNameWithApp(testVolName, "myapp")
	if IsArchivedShareName(active) {
		t.Fatalf("active share %q reported as archived", active)
	}
	if !IsArchivedShareName(GenArchivedShareName(active)) {
		t.Fatalf("archived form of %q not reported as archived", active)
	}
}
