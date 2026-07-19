// Copyright 2021 Synology Inc.

package utils

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

type AuthType string

const (
	UNIT_GB = 1024 * 1024 * 1024
	UNIT_MB = 1024 * 1024

	ProtocolSmb     = "smb"
	ProtocolIscsi   = "iscsi"
	ProtocolNfs     = "nfs"
	ProtocolNvme    = "nvme"
	ProtocolDefault = ProtocolIscsi

	AuthTypeReadWrite AuthType = "rw"
	AuthTypeReadOnly  AuthType = "ro"
	AuthTypeNoAccess  AuthType = "no"
)

// managedShareRe matches the only shared folders this driver is ever allowed to
// modify or delete: active volumes (k8s-csi-pvc-…) and archived ones that were
// created by the driver and later renamed on delete (del-csi-pvc-…).
//
// The "csi-pvc" token is a deliberately fixed, non-configurable safety anchor: a
// configurable prefix could be set to an empty or overly broad value and would
// then match every shared folder on the volume.
var managedShareRe = regexp.MustCompile(`^(k8s|del)-csi-pvc(-|$)`)

// IsManagedShare reports whether a DSM shared folder belongs to this driver.
func IsManagedShare(shareName string) bool {
	return managedShareRe.MatchString(shareName)
}

// AssertManagedShare fails closed: it returns an error unless the shared folder is
// one this driver created. Call it as a precondition of every mutating DSM share
// operation so a bug or an unexpected name can never touch a user's own data.
func AssertManagedShare(op string, shareName string) error {
	if IsManagedShare(shareName) {
		return nil
	}
	return fmt.Errorf(
		"refusing to %s shared folder %q: not managed by this driver (must match %s)",
		op, shareName, managedShareRe.String())
}

func SliceContains(items []string, s string) bool {
	for _, item := range items {
		if s == item {
			return true
		}
	}
	return false
}

func MBToBytes(size int64) int64 {
	return size * UNIT_MB
}

func BytesToMB(size int64) int64 {
	return size / UNIT_MB
}

// Ceiling
func BytesToMBCeil(size int64) int64 {
	return (size + UNIT_MB - 1) / UNIT_MB
}

func StringToBoolean(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "yes" || value == "true" || value == "1"
}

func StringToSlice(value string) []string {
	return strings.Fields(value)
}

func BoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// Haven't supported IPv6 yet.
func LookupIPv4(name string) ([]string, error) {
	ips, _ := net.LookupIP(name)

	retIps := []string{}
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			retIps = append(retIps, ipv4.String())
		}
	}
	if len(retIps) > 0 {
		return retIps, nil
	}

	return nil, fmt.Errorf("Failed to LookupIPv4 by local resolver for: %s", name)
}
