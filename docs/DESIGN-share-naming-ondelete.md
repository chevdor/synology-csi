# Design: share naming, `onDelete` lifecycle, and archive purge

Fork-local design for `csi.san.synology.com` (SMB/NFS share-backed volumes).
Status: **design agreed, one open item (rename API probe) before implementation.**

## Context / problem

The driver creates one DSM shared folder per PV, named `k8s-csi-pvc-<uuid>` (capped at
`MaxShareLen = 32`). Three pain points in real use:

1. **Opaque names** — a folder list in DSM is a wall of UUIDs; you cannot tell which app owns
   which share.
2. **Orphans are indistinguishable** — with `reclaimPolicy: Retain`, deleting a PVC leaves the
   share untouched and *identically named* to an active one, so there is no safe way to tell
   what can be removed. Released PVs also pile up in k8s.
3. **No k8s-side purge** — cleaning archived shares requires logging into the NAS.

Goal: readable names, an explicit archived state, and full lifecycle control from Kubernetes.

> Related, already shipped on this fork: `enableRecycleBin` StorageClass parameter
> (Recycle Bin was hardcoded on; it silently consumed the share quota until emptied).

## 🛑 Hard safety invariant (non-negotiable)

**The driver must NEVER create, rename, modify, or delete any share whose name does not match:**

```
^(k8s|del)-csi-pvc(-|$)
```

No exception, no code path, no parameter, no configuration. Any share outside this pattern —
every pre-existing user share on the volume — is strictly out of scope and must never be
touched, read-modified, renamed, or deleted.

**Enforcement — fail closed, at every mutating call site** (not only at discovery):

| call site | why it matters |
|---|---|
| `ShareDelete` | destructive |
| `ShareSet` (quota, Description) | mutates an existing share |
| rename (`archive`) | mutates an existing share |
| **purge path (`deleteOnUpdate`)** | **highest risk** — it deliberately widens lookup beyond active discovery to reach `del-` shares |

Implement as a single helper (e.g. `models.IsManagedShare(name) bool` / `AssertManagedShare`)
called as a **precondition** in every mutating DSM share operation. If the name does not match:
return an error and log loudly — never silently continue, never "best effort".

**Consequence for the naming design:** the `csi-pvc` token is a **fixed, non-configurable
safety anchor**, and `status` is a closed enum of exactly `k8s` and `del`. A free-form
configurable prefix is rejected on purpose: a misconfigured or empty prefix would widen the
guard to match every share on the volume.

## Naming scheme (decided)

```
active:    k8s-csi-pvc[-<name>]-<uuid>
archived:  del-csi-pvc[-<name>]-<uuid>
```

| segment | len | notes |
|---|---|---|
| `status` | 3 | `k8s` = active, `del` = archived (PVC deleted, folder kept) |
| `csi-pvc` | 7 | fixed type/discovery token |
| `name` | ≤ 10 | **optional**, truncated PVC name |
| `uuid` | 9 | slice of the PV UUID — **the lookup key** |

Budget: fixed part `k8s-csi-pvc-` = 12 chars, leaving **19** for `name` + `-` + `uuid`
(10 + 1 + 9 = 20 → name capped at 10 with uuid 9, total 32). *Chosen: Option B — roomier name.*

- **When `name` is omitted its budget returns to the uuid** (up to 20 chars), i.e. today's
  behaviour, which keeps existing shares resolvable.
- **Uniqueness:** 9 hex ≈ 6.8e10 values; at ~512 shares collision probability ≈ 1e-6. Fine at
  homelab scale. *Never* truncate the uuid to make room — truncate the name.
- Archive is a **prefix swap only** (`k8s` → `del`), so the uuid/lookup key is preserved.

## `onDelete` lifecycle

New StorageClass parameter `onDelete: delete | archive` (only consulted when `DeleteVolume` is
actually invoked, i.e. `reclaimPolicy: Delete`).

- **`delete`** — current behaviour: `ShareDelete`.
- **`archive`** — rename status prefix `k8s` → `del`. Data is kept, the folder is obviously
  marked, and because discovery filters on `k8s-csi-pvc`, the share **drops out of the driver's
  view automatically** (verified by hand: a share renamed to `del-…` is no longer matched).

This is strictly better than plain `Retain`: data kept **and** identifiable **and** no Released
PVs accumulating.

## Purge — `deleteOnUpdate: true | false` (driver option)

**Default: `false` — safe by default.** Purging is destructive and therefore opt-in; the driver
never removes archived data unless explicitly told to.

- **`false`** (default) — ignore; `del-` shares are left on the NAS for manual review/cleanup.
- **`true`** — a delete/update request resolving to an **archived (`del-`)** share will really
  delete it → archived shares can be purged **from Kubernetes, without logging into the NAS**.

Rationale: the whole point of `archive` is that deleting a PVC must not destroy data. A purge
path that is on by default would quietly re-introduce exactly that risk.

**Mechanism / why code changes are needed:** `DeleteVolume` resolves a share via
`GetVolume(volId)` → `ListVolumes()` → `listSMBorNFSVolumes`, which filters
`HasPrefix(share.Name, SharePrefix)`. Archived `del-` shares are therefore **unreachable**
today. Purge requires the delete-path lookup to resolve a share **by its DSM UUID across both
`k8s-` and `del-` prefixes** (the UUID is stable across rename), gated by `deleteOnUpdate`.

## Correctness constraints (the landmines)

1. **Idempotency.** `GetVolumeByName(volName)` matches
   `volume.Name == GenShareName(volName)`. With an optional `name` segment (which comes from
   StorageClass/PVC *params*, not from `volName`) the name can no longer be reconstructed from
   `volName` alone → risk of duplicate shares on retry.
   **Fix:** match on the **`-<uuid>` suffix**, which *is* derivable from `volName`. The `name`
   segment stays purely cosmetic and is never used for matching.
2. **Discovery.** Keep `k8s-csi-pvc` as the active token so archived shares are excluded *by
   construction*. The `csi-pvc` anchor is **fixed and not configurable** (see the hard safety
   invariant) — a per-StorageClass or free-form prefix is rejected, since global discovery could
   not then know what is "ours", and a bad value would widen the safety guard.
3. **Uniqueness.** Truncate `name`, never `uuid`.
4. **Backward compatibility.** Existing shares are `k8s-csi-pvc-<uuid20>` with no name segment;
   uuid-suffix matching must still resolve them for both idempotency and delete.

## Rename over the DSM API — verification

Renaming at the **DSM level is confirmed**: a manual UI rename
(`k8s-csi-pvc-f0601a65-…` → `del-csi-pvc-f0601a65-…`) preserved the data, and the renamed folder
correctly dropped out of driver discovery.

The **API path** is what `archive` depends on. Candidate: `ShareSet`
(`SYNO.Core.Share` `set`, already used for quota updates) with a changed `name`.

> **STATUS: to be verified by the probe (step 1 of the test plan). Replace this block with the
> result — the working API call and any constraints — once the probe has run.**

If rename proves unsupported over the API, the fallback is to tag the share **Description**
via `ShareSet` (proven to work), accepting that Description shows only in the folder *detail*,
not the folder list.

## Test plan

1. **Probe** DSM rename via API on a throwaway share (blocking).
2. **Naming** — PVC with a name → `k8s-csi-pvc-<name>-<uuid>`; without → `k8s-csi-pvc-<uuid>`.
3. **Archive** — delete PVC with `onDelete: archive` → renamed `del-…`, data intact, gone from
   `kubectl get pv` and from driver discovery.
4. **Purge** — `deleteOnUpdate: true` removes the `del-` share from k8s; `false` leaves it.
5. **Backward compat** — a pre-existing `k8s-csi-pvc-<uuid20>` share still resolves for
   idempotency and delete.
6. **Uniqueness** — provision many PVCs; assert no name collisions and correct lookups.

Build/verify loop: build the driver image for your node architecture → push to your container
registry → point the CSI **controller** at the new image → verify the resulting share in DSM.
