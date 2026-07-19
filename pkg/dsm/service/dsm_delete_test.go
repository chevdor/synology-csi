/*
 * Copyright 2022 Synology Inc.
 */

package service

import (
	"testing"

	"github.com/SynologyOpenSource/synology-csi/pkg/models"
)

const (
	activeShare   = "k8s-csi-pvc-myapp-729da2eaf"
	archivedShare = "del-csi-pvc-myapp-729da2eaf"
)

// The delete path is the only one that destroys data, so every combination is
// pinned here rather than relying on manual testing against a live DSM.
func TestDecideShareDeleteAction(t *testing.T) {
	cases := []struct {
		name          string
		shareName     string
		onDelete      string
		purgeArchived bool
		want          shareDeleteAction
	}{
		// Default policy: destroy the folder, as upstream does.
		{"active + delete policy", activeShare, models.OnDeleteDelete, false, shareActionDelete},

		// Archive policy: keep the data, rename it out of the driver's scope.
		{"active + archive policy", activeShare, models.OnDeleteArchive, false, shareActionArchive},

		// Purge: only reachable when the caller resolved the volume as archived.
		{"archived + purge requested", archivedShare, models.OnDeleteArchive, true, shareActionPurge},
		{"archived + purge, delete policy", archivedShare, models.OnDeleteDelete, true, shareActionPurge},

		// THE REGRESSION: a live share must never be destroyed by the purge branch,
		// even if the archived lookup wrongly handed it over.
		{"active wrongly offered to purge", activeShare, models.OnDeleteArchive, true, shareActionRefuse},
		{"active wrongly offered to purge, delete policy", activeShare, models.OnDeleteDelete, true, shareActionRefuse},

		// An already-archived share must not be archived a second time.
		{"archived + archive policy, no purge", archivedShare, models.OnDeleteArchive, false, shareActionDelete},

		// Unknown policy behaves as the safe upstream default rather than archiving.
		{"unknown policy", activeShare, "bogus", false, shareActionDelete},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decideShareDeleteAction(c.shareName, c.onDelete, c.purgeArchived)
			if got != c.want {
				t.Fatalf("decideShareDeleteAction(%q, %q, %v) = %v, want %v",
					c.shareName, c.onDelete, c.purgeArchived, got, c.want)
			}
		})
	}
}

// Whatever the inputs, a live share may never end up destroyed by the purge path.
func TestPurgeNeverTargetsLiveShare(t *testing.T) {
	for _, policy := range []string{models.OnDeleteDelete, models.OnDeleteArchive, "bogus"} {
		if got := decideShareDeleteAction(activeShare, policy, true); got != shareActionRefuse {
			t.Fatalf("purge of a live share with policy %q returned %v, want refuse", policy, got)
		}
	}
}
