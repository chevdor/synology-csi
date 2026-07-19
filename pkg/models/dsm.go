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
