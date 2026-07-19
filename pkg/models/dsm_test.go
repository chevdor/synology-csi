// Copyright 2021 Synology Inc.

package models

import "testing"

func TestGenArchivedShareName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"active share is archived", "k8s-csi-pvc-3b919ccaaece4893", "del-csi-pvc-3b919ccaaece4893"},
		{"with app name", "k8s-csi-pvc-myapp-3b919cca", "del-csi-pvc-myapp-3b919cca"},
		{"already archived is untouched", "del-csi-pvc-3b919cca", "del-csi-pvc-3b919cca"},
		{"unmanaged name is untouched", "photo", "photo"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GenArchivedShareName(c.in)
			if got != c.want {
				t.Fatalf("GenArchivedShareName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// The archived name must stay within the DSM shared-folder limit. Since archiving
// only swaps a 3-char status prefix, length is preserved — so a name that fit
// before still fits after.
func TestGenArchivedShareNamePreservesLength(t *testing.T) {
	// Longest possible active name: exactly MaxShareLen.
	in := GenShareName("pvc-3189b84c-04e1-4eb4-af07-9429695ba97e")
	if len(in) != MaxShareLen {
		t.Fatalf("precondition: expected a %d-char name, got %d (%q)", MaxShareLen, len(in), in)
	}

	out := GenArchivedShareName(in)
	if len(out) != len(in) {
		t.Fatalf("archiving changed length: %d -> %d (%q -> %q)", len(in), len(out), in, out)
	}
	if len(out) > MaxShareLen {
		t.Fatalf("archived name %q exceeds MaxShareLen %d", out, MaxShareLen)
	}
}
