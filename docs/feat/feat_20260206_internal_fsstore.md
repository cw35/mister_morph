---
date: 2026-02-06
title: Internal FSStore Prerequisite (MAEP + Contacts)
status: done
---

# Internal FSStore Prerequisites and Implementation Plan

## 1) Goal

This requirement does one thing only: provide a unified, reusable file persistence foundation for migrating the current `maep` implementation and for future reuse by contacts business storage.

Domain consumers:
- `maep` (already exists, migrate first).
- `guard` (already exists, migrate JSONL writer).
- `contacts` (future integration, reuse file capabilities).

Directory conventions (required):
- `maep` has its own `maep_dir` (already exists).
- `guard` has its own `guard_dir` (new requirement).
- `contacts` has its own `contacts_dir` (future implementation).

Hard objectives:
- Migrate low-level file I/O in `maep/file_store.go` to `internal/fsstore`.
- Migrate `guard/audit_jsonl.go` to `internal/fsstore.JSONLWriter` (no behavior change).
- Migrate `guard approvals` from SQLite to a file backend (based on `internal/fsstore`).
- Standardize audit persistence as JSONL (`maep` and `guard`).
- Provide atomic writes, locking, and JSONL append capabilities for future contacts usage.
- Provide minimal generic indexing (generic key-value index file) to satisfy basic "fast access by key" requirements.

Constraints:
- No database introduced.
- No new business abstraction layer introduced.
- Extract only file capabilities, not domain models.

## 2) Scope

### 2.1 in scope
- Directory creation and permission constraints.
- JSON read/write (read, atomic write).
- Text read/write (for `active.md` / `inactive.md`).
- JSONL append and rotation.
- Generic index file read-modify-write (key -> entry, no domain semantics).
- Cross-process locking (same host).
- Error semantics and basic testing utilities.

### 2.2 out of scope
- `maep.Store` domain interface definition.
- Contacts business schema (including `contacts/index.json` structure).
- Any business policy (scoring, eviction, trust rules, prompt decisions).
- Distributed locks.

## 3) Current Problems (Migration Motivation)

Sources of duplicated implementation:
- `maep/file_store.go`: JSON read/write, atomic write, directory permissions, in-process lock.
- `guard/audit_jsonl.go`: JSONL append, rotation, writer lifecycle.

Risks:
- Inconsistent permission and atomicity strategies.
- Inconsistent cross-process concurrency semantics (`sync.Mutex` is in-process only).
- Reimplementing for contacts would continue divergence.

## 4) Target Package and Suggested Layout

Frozen package name: `internal/fsstore`

Suggested file layout:
- `internal/fsstore/options.go`
- `internal/fsstore/atomic.go`
- `internal/fsstore/json.go`
- `internal/fsstore/text.go`
- `internal/fsstore/jsonl.go`
- `internal/fsstore/index.go`
- `internal/fsstore/lock.go`
- `internal/fsstore/errors.go`

Notes:
- Start with a functional API to avoid premature class hierarchy.
- Add object-oriented forms later (for example a `Store` struct) if needed, without blocking current migration.

## 5) API v1 (Frozen Draft)

```go
package fsstore

import (
    "encoding/json"
    "context"
    "os"
    "time"
)

type FileOptions struct {
    DirPerm  os.FileMode // default 0700
    FilePerm os.FileMode // default 0600
}

type JSONLOptions struct {
    DirPerm        os.FileMode // default 0700
    FilePerm       os.FileMode // default 0600
    RotateMaxBytes int64       // default 100 MiB
    FlushEachWrite bool        // default true
    SyncEachWrite  bool        // default false
}

func EnsureDir(path string, perm os.FileMode) error

func ReadJSON(path string, out any) (exists bool, err error)
func WriteJSONAtomic(path string, v any, opts FileOptions) error

func ReadText(path string) (content string, exists bool, err error)
func WriteTextAtomic(path string, content string, opts FileOptions) error

type IndexEntry struct {
    Ref       string          `json:"ref,omitempty"`
    Rev       uint64          `json:"rev,omitempty"`
    Hash      string          `json:"hash,omitempty"`
    UpdatedAt time.Time       `json:"updated_at,omitempty"`
    Meta      json.RawMessage `json:"meta,omitempty"` // domain-defined metadata
}

type IndexFile struct {
    Version int                   `json:"version"`
    Entries map[string]IndexEntry `json:"entries"`
}

func ReadIndex(path string) (index IndexFile, exists bool, err error)
func WriteIndexAtomic(path string, index IndexFile, opts FileOptions) error
func MutateIndex(ctx context.Context, path string, lockPath string, opts FileOptions, fn func(*IndexFile) error) error

func BuildLockPath(lockRoot string, lockKey string) (string, error)
func WithLock(ctx context.Context, lockPath string, fn func() error) error

type JSONLWriter struct { /* unexported */ }
func NewJSONLWriter(path string, opts JSONLOptions) (*JSONLWriter, error)
func (w *JSONLWriter) AppendJSON(v any) error
func (w *JSONLWriter) AppendLine(line string) error // line must not contain '\n'
func (w *JSONLWriter) Close() error
```

API notes:
- `ReadJSON` / `ReadText`: when file does not exist, return `exists=false, err=nil`.
- `Write*Atomic`: guarantees the target path is either old version or new version, never partially written.
- `IndexFile` is a generic index container with no domain rules; domain fields go into `Meta`.
- `MutateIndex` provides integrated read-modify-write + lock to avoid repeated caller boilerplate.
- `BuildLockPath` validates lock key and maps it to a hidden lock directory path.
- `WithLock`: same-host cross-process mutual exclusion; lock scope is determined by `lockPath`.

## 6) Semantic Details (Must Follow)

### 6.1 Permissions
- Default directory permission `0700`, default file permission `0600`.
- For all writes, call `EnsureDir(filepath.Dir(path))` before writing.
- After atomic write completes, target file permissions must be `FilePerm`.

### 6.2 Atomic write flow
Consistent for `WriteJSONAtomic` / `WriteTextAtomic`:
1. Create a temp file in the target directory (same directory).
2. Write full content.
3. `fsync` the temp file.
4. Set temp file permission.
5. `rename(temp, target)` as atomic replacement.
6. Best-effort `fsync` directory (warn only on failure, no rollback).
7. Clean up temp file on failure paths.

### 6.3 JSON encoding
- Use UTF-8.
- Default to `MarshalIndent` (2 spaces) and append trailing `\n` to reduce manual review noise.
- No schema validation in `fsstore`; callers are responsible.

### 6.4 JSONL spec
- One JSON object per line, with trailing `\n`.
- Before append, if `current_size + incoming_bytes > RotateMaxBytes`, rotate first then write.
- Rotation filename: `<path>.YYYYMMDDTHHMMSSZ` (UTC).
- If rotation target already exists (same-second collision), auto-append suffix `.<n>` (starting at `1`) until success.
- With `FlushEachWrite=true`, flush after each append.
- With `SyncEachWrite=true`, `fsync` after each append (high-reliability audit scenarios only).

### 6.5 Lock strategy (decided)
- Domain-level primary lock:
  - MAEP: key=`state.main`
  - Contacts: key=`state.main`
- Audit-stream lock: one lock key per audit file (for example `audit.audit_events_jsonl`).

Lock key naming pattern (unified):
- Two-part format: `<scope>.<resource>`
- Common `scope`: `state`, `audit`, `index`
- Lock key represents a "resource mutex unit", not separated by business action.
- Actions like `append/rotate` must reuse the same audit-stream lock key to avoid races from dual locks.

Lock path rules (avoid exposing business names directly in filenames):
- First determine `lockRoot`:
  - MAEP: `<maep_dir>/.fslocks`
  - Guard: `<guard_dir>/.fslocks`
  - Contacts: `<contacts_dir>/.fslocks`
- Lock key spec:
  - Allowed charset only: `[a-z0-9._-]`
  - Lowercase only
  - No leading `.` and no trailing `.`
  - Max length `<= 120`
- Final lock file path: `<lockRoot>/<lockKey>.lck`
- Lock key rules are frozen in this version. Any future rule change requires downtime migration and a guarantee that only one rule set runs at a time.

`.lck` file content convention:
- Correctness relies on OS file lock (`flock/fcntl`) + open file descriptor ownership, not file content.
- Content is diagnostic only. Recommended one-line JSON:
  - `{"lock_key":"...","pid":1234,"hostname":"...","acquired_at":"...","owner":"<random-id>"}`
- File content is for observability/troubleshooting only, never for stale-lock decisions.

Examples:
- `lockKey = "state.main"` -> `<maep_dir>/.fslocks/state.main.lck`
- `lockKey = "audit.audit_events_jsonl"` -> `<maep_dir>/.fslocks/audit.audit_events_jsonl.lck`
- `lockKey = "audit.guard_audit_jsonl"` -> `<guard_dir>/.fslocks/audit.guard_audit_jsonl.lck`

Execution constraints:
- State mutations must run under the corresponding domain-level lock.
- Normal reads are lock-free by default. For cross-file strongly consistent reads, readers must use the same resource lock key as writers (for example `state.main`).
- Audit can be appended within the domain lock or independently. If one operation writes both state and audit, fixed order is "state first, audit second".
- Nested acquisition of the same lock path is forbidden to avoid self-deadlock.

Platform implementation constraints:
- Unix-like (Linux/macOS): implement `WithLock` using `flock/fcntl`.
- Windows (degraded implementation): lock-file occupancy via `os.O_CREATE|os.O_EXCL` + polling retry.
- `WithLock` wait duration is controlled only by `ctx`; no additional stale-lock auto-recovery.
- Under Windows degraded mode, lock files left by abnormal exits must be cleaned up manually (`.lck` file).

### 6.6 Error semantics
Suggested error categories (sentinel + wrap):
- `ErrInvalidPath`
- `ErrLockTimeout` (when `ctx` times out/cancels)
- `ErrLockUnavailable`
- `ErrEncodeFailed`
- `ErrDecodeFailed`
- `ErrAtomicWriteFailed`

Notes:
- Business layer can route retry/alert strategy via `errors.Is`.

## 7) Migration Targets and Steps

### 7.1 Phase A: deliver fsstore core capabilities (completed)
- Implement API v1.
- Complete unit tests and concurrency tests.

### 7.2 Phase B: migrate guard JSONL (completed)
- Add guard-specific directory convention:
  - Config: `guard.dir_name` (default `guard`).
  - Resolution: `guard_dir = <file_state_dir>/<guard.dir_name>`.
  - Default audit path: `<guard_dir>/audit/guard_audit.jsonl`.
  - Default approvals file path: `<guard_dir>/approvals/guard_approvals.json`.
- Refactor `guard/audit_jsonl.go` to reuse `fsstore.JSONLWriter`.
- Unify guard lock and path strategy through `BuildLockPath + WithLock`.
- Migrate guard approvals from SQLite to file backend (JSON + atomic write + lock).
- Freeze guard approvals file format:
  - Top-level: `{ "version": 1, "records": { "<approval_id>": { ...ApprovalRecord... } } }`
  - `records` uses `approval_id -> record` map for O(1) `Get/Resolve`.
- No automatic migration; historical SQLite data is migrated manually.
- `guard.approvals.sqlite_dsn` config key removed (no longer read).
- No separate approvals audit stream in this phase (no `guard_approvals_audit.jsonl`).

### 7.3 Phase C: migrate maep file store (completed)
- Migrate `readJSONFile`, `writeJSONFileAtomic`, and directory creation logic from `maep/file_store.go` to `fsstore`.
- Keep in-process `sync.Mutex` for object-level concurrency; add cross-process lock at file level.

### 7.4 Phase D: upgrade maep audit file to JSONL (completed)
- Migrate from `audit_events.json` to `audit_events.jsonl`.
- Compatibility strategy:
  1. Prefer reading `.jsonl`.
  2. If `.jsonl` does not exist and `.json` exists, import once and generate `.jsonl`.
  3. After successful import, rename old `.json` to `audit_events.json.migrated.<ts>` (do not delete directly).

### 7.5 Phase E: contacts integration prep (pending)
- Contacts business layer uses `WriteTextAtomic` for `active.md` / `inactive.md`.
- Contacts audit uses `JSONLWriter` directly.
- Whether `contacts/index.json` exists and its structure are defined by contacts business docs, not frozen in this phase.

## 8) Test Matrix (Must Cover)

### 8.1 Atomic write
- After interrupted writes, target file remains readable (old or new version).
- High-frequency overwrite does not produce corrupted JSON.

### 8.2 Lock
- Two processes concurrently writing the same target are serialized.
- `ctx` cancellation exits lock wait in time.

### 8.3 JSONL
- Appended lines are complete, no partial line.
- Appending continues after rotation.
- Concurrent append has no interleaved corruption.

### 8.4 MAEP regression
- No behavior change in contact/inbox/dedupe/protocol_history.
- `maep audit list` output and filtering remain compatible (only storage medium changes).

## 9) TODO (Implementation Checklist)

### 9.1 fsstore package
- [x] Create `internal/fsstore` package layout.
- [x] Implement `EnsureDir`.
- [x] Implement `ReadJSON` / `WriteJSONAtomic`.
- [x] Implement `ReadText` / `WriteTextAtomic`.
- [x] Implement `ReadIndex` / `WriteIndexAtomic` / `MutateIndex`.
- [x] Implement `BuildLockPath` (validate lock key -> hidden lock file path).
- [x] Implement `WithLock(context, lockPath, fn)`.
- [x] Implement `JSONLWriter` and rotation rules.
- [x] Add sentinel errors and `errors.Is` semantics.
- [x] Add Windows degraded lock implementation and documentation.

### 9.2 migrate guard
- [x] Add `guard.dir_name` config (default `guard`).
- [x] Switch default paths to `guard_dir`-based (audit + approvals).
- [x] Refactor `guard/audit_jsonl.go` to reuse `internal/fsstore`.
- [x] Migrate approvals from SQLite to file backend (`guard_approvals.json`).
- [x] Remove `guard.approvals.sqlite_dsn` config key.
- [x] Do not add a separate approvals audit stream.

### 9.3 migrate maep
- [x] Replace low-level I/O helper functions in `maep/file_store.go`.
- [x] Add domain-level lock integration (`state.main` -> `BuildLockPath`).
- [x] Add `audit_events.json -> audit_events.jsonl` migration logic.
- [x] Regress `maep` related tests.

### 9.4 reserve for contacts
- [x] Verify `WriteTextAtomic` satisfies markdown main-file updates (when contacts integrates).
- [x] Verify contacts audit JSONL append capability (when contacts integrates).

## 10) Acceptance Criteria

This prerequisite is considered complete when all conditions below are met:
- `maep` and `guard` no longer maintain duplicate atomic-write/JSONL logic.
- `maep` and `guard` audits are both JSONL.
- `maep` file read/write has cross-process lock guarantees.
- Contacts can directly reuse `internal/fsstore` for file-based storage.
- `ReadIndex` / `WriteIndexAtomic` / `MutateIndex` are implemented with test coverage.
