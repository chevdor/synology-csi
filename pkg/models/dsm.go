// Copyright 2021 Synology Inc.

package models

import (
	"fmt"
	"strings"
)

const (
	K8sCsiName       = "Kubernetes CSI"

	// ISCSI definitions
	FsTypeExt4       = "ext4"
	FsTypeBtrfs      = "btrfs"
	LunTypeFile      = "FILE"
	LunTypeAdv       = "ADV"
	LunTypeBlun      = "BLUN"               // thin provision, mapped to type 263
	LunTypeBlunThick = "BLUN_THICK"         // thick provision, mapped to type 259
	MaxIqnLen = 128

	// NVMe definitions
	MaxNqnLen = 128

	// Share definitions
	MaxShareLen     = 32
	MaxShareDescLen = 64
	UserGroupTypeLocalUser  = "local_user"
	UserGroupTypeLocalGroup = "local_group"
	UserGroupTypeSystem     = "system"


	// CSI definitions
	TargetPrefix            = "k8s-csi"
	SubsystemPrefix         = TargetPrefix
	DevicePrefix            = "k8s-csi"
	IqnPrefix               = "iqn.2000-01.com.synology:"
	NqnPrefix               = "nqn.2000-01.com.synology:"
	SharePrefix             = "k8s-csi"
	ShareSnapshotDescPrefix = "(Do not change)"
)

func GenBackendName(volName string) string {
	return fmt.Sprintf("%s-%s", DevicePrefix, volName)
}

func GenShareName(volName string) string {
	shareName := fmt.Sprintf("%s-%s", SharePrefix, volName)
	if len(shareName) > MaxShareLen {
		return shareName[:MaxShareLen]
	}
	return shareName
}

// Share naming.
//
// Layout: {status}-csi-pvc[-{app}]-{uuid}, within MaxShareLen (32).
//
//	k8s-csi-pvc-myapp-729da2eaf
//	└─ 11 ─┘ └ ≤10 ┘ └── 9 ──┘
//
// The trailing uuid is the *lookup key*: it is derived from the CSI volume name
// alone, so a share can still be found even though the {app} segment is not
// recoverable from it. {app} is cosmetic and is what gets truncated when space
// runs out — the uuid never is, because shortening it would weaken uniqueness.
const (
	ShareUUIDLen  = 9  // hex chars of the PV UUID kept as the lookup key
	MaxAppNameLen = 10 // cosmetic app segment budget
)

// ShareUUIDSlice derives the stable lookup key from a CSI volume name, which
// Kubernetes sets to the PV name ("pvc-<uuid>"). Hyphens are stripped so the key
// is pure hex and a fixed length.
func ShareUUIDSlice(volName string) string {
	hex := strings.ReplaceAll(strings.TrimPrefix(volName, "pvc-"), "-", "")
	if len(hex) > ShareUUIDLen {
		return hex[:ShareUUIDLen]
	}
	return hex
}

// sanitizeAppName reduces a PVC name to characters that are safe in a DSM shared
// folder name.
func sanitizeAppName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// GenShareNameWithApp builds {status}-csi-pvc[-{app}]-{uuid} for a volume,
// embedding a readable (truncated) app name. Falls back to the app-less form when
// there is no usable app name.
func GenShareNameWithApp(volName string, appName string) string {
	base := SharePrefix + "-pvc" // k8s-csi-pvc
	uuid := ShareUUIDSlice(volName)

	app := sanitizeAppName(appName)
	if app != "" {
		budget := MaxShareLen - len(base) - len(uuid) - 2 // two separators
		if budget > MaxAppNameLen {
			budget = MaxAppNameLen
		}
		if budget > 0 {
			if len(app) > budget {
				app = app[:budget]
			}
			app = strings.Trim(app, "-")
		} else {
			app = ""
		}
	}

	if app == "" {
		return fmt.Sprintf("%s-%s", base, uuid)
	}
	return fmt.Sprintf("%s-%s-%s", base, app, uuid)
}

// ShareNameMatchesVolume reports whether a shared folder belongs to the given CSI
// volume. It accepts both the current scheme (matched on the uuid suffix) and the
// legacy scheme produced by GenShareName, so shares provisioned before the naming
// change stay resolvable — otherwise every existing volume would be orphaned.
func ShareNameMatchesVolume(shareName string, volName string) bool {
	if shareName == GenShareName(volName) {
		return true
	}
	return strings.HasSuffix(shareName, "-"+ShareUUIDSlice(volName))
}

// Status prefixes for driver-created shares. Active shares are discovered via
// SharePrefix; archived ones deliberately fall outside it so the driver stops
// managing them.
const (
	ShareStatusActive   = "k8s"
	ShareStatusArchived = "del"
)

// onDelete policies for share-backed (SMB/NFS) volumes.
//
// This is a driver-level setting rather than a StorageClass parameter because the
// CSI spec only delivers StorageClass parameters to CreateVolume; DeleteVolume
// receives just a volume id, so the driver cannot learn a per-class policy at the
// moment it matters.
const (
	OnDeleteDelete  = "delete"  // destroy the shared folder (upstream behaviour)
	OnDeleteArchive = "archive" // keep it, renamed k8s-… -> del-…
)

// ArchivedSharePrefix is to archived shares what SharePrefix is to active ones
// (k8s-csi -> del-csi). Derived rather than hardcoded so the two cannot drift.
var ArchivedSharePrefix = ShareStatusArchived + strings.TrimPrefix(SharePrefix, ShareStatusActive)

// IsArchivedShareName reports whether a share has been archived (belongs to a
// deleted PVC). Callers that destroy data must check this explicitly: "managed by
// the driver" is NOT the same as "archived", and conflating the two risks deleting
// a live volume instead of archiving it.
func IsArchivedShareName(shareName string) bool {
	return strings.HasPrefix(shareName, ArchivedSharePrefix)
}

// GenArchivedShareName swaps the 3-char status prefix, k8s-csi-pvc-… -> del-csi-pvc-….
// Renaming keeps the folder (and its data) but marks it as belonging to a deleted
// PVC, and drops it out of discovery since that matches SharePrefix.
// The name length is unchanged, so the DSM 32-char limit still holds.
func GenArchivedShareName(shareName string) string {
	if !strings.HasPrefix(shareName, ShareStatusActive) {
		return shareName
	}
	return ShareStatusArchived + strings.TrimPrefix(shareName, ShareStatusActive)
}
