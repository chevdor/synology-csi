// Copyright 2021 Synology Inc.

package utils

import "testing"

func TestIsManagedShare(t *testing.T) {
	cases := []struct {
		name  string
		share string
		want  bool
	}{
		// Shares the driver created — may be modified/deleted.
		{"active volume", "k8s-csi-pvc-3b919ccaaece4893", true},
		{"active with app name", "k8s-csi-pvc-myapp-3b919cca", true},
		{"archived volume", "del-csi-pvc-3b919ccaaece4893", true},
		{"bare prefix, no suffix", "k8s-csi-pvc", true},

		// Everything else must be refused — these are a user's own shares.
		{"typical user share", "photo", false},
		{"another user share", "homes", false},
		{"documents", "documents", false},
		{"empty name", "", false},

		// Near-misses that must NOT be treated as ours.
		{"no separator after pvc", "k8s-csi-pvcsomething", false},
		{"prefixed by something else", "xk8s-csi-pvc-abc", false},
		{"different token", "k8s-csi-vol-abc", false},
		{"unknown status prefix", "rtn-csi-pvc-abc", false},
		{"uppercase", "K8S-CSI-PVC-abc", false},
		{"substring match only", "backup-k8s-csi-pvc-abc", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsManagedShare(c.share); got != c.want {
				t.Fatalf("IsManagedShare(%q) = %v, want %v", c.share, got, c.want)
			}
		})
	}
}

func TestAssertManagedShare(t *testing.T) {
	if err := AssertManagedShare("delete", "k8s-csi-pvc-abc"); err != nil {
		t.Fatalf("expected managed share to be allowed, got %v", err)
	}

	// Fail closed on anything the driver did not create.
	err := AssertManagedShare("delete", "photo")
	if err == nil {
		t.Fatal("expected an error for an unmanaged share, got nil")
	}
	if !contains(err.Error(), "photo") || !contains(err.Error(), "delete") {
		t.Fatalf("error should name the operation and the share, got: %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
