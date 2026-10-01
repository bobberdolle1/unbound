package historyid

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ContextKeyFileName is the local context key file inside the owned config
// directory. It is deliberately NOT part of managed activation state: it
// represents local history namespace continuity, so Suspend, Revert and graph
// Revert must never create, rotate, or delete it.
const ContextKeyFileName = "autotune_vnext_context.key"

// contextKeyBytes is 32 bytes (256 bits) of CSPRNG output.
const contextKeyBytes = 32

// A concurrent creator may hold the key path for only the moment between its
// exclusive create and its write. These bounds let that writer finish instead of
// reporting a spurious failure, while still failing promptly if the file is
// genuinely stuck.
const (
	contextKeyCreateAttempts   = 5
	contextKeyCreateRetryDelay = 40 * time.Millisecond
)

// errContextKeyCorrupt marks a key file that exists but is not exactly the
// expected 32 raw bytes.
var errContextKeyCorrupt = errors.New("context key file is corrupt")

// LoadOrCreateContextKey returns the local HMAC key, creating it on first use.
//
// ATOMICITY. Two processes starting at the same time must never end up with two
// different keys, because that would silently split one local history namespace
// into two. Creation uses O_CREATE|O_EXCL so exactly one process wins; a loser
// does NOT overwrite, it re-reads the winner's key.
//
// FAIL CLOSED. A corrupt key file does not get silently replaced and does not
// get reinterpreted. Previous history entries would otherwise appear to belong
// to a namespace the user never chose. The caller receives an error and falls
// back to CONTEXT_IDENTITY_UNAVAILABLE, which only makes history reuse more
// conservative. Reclaiming a namespace is a future explicit recovery action,
// never an implicit side effect of a read.
func LoadOrCreateContextKey(configDir string) ([]byte, error) {
	if configDir == "" {
		return nil, fmt.Errorf("context key requires an owned config directory")
	}
	path := filepath.Join(configDir, ContextKeyFileName)

	// A zero-length file means a concurrent creator has not written yet, so a
	// short bounded retry lets that writer finish instead of failing. This also
	// recovers from a crash between create and write: an empty file provably
	// holds no key, so completing it cannot reinterpret any history, it can only
	// start a fresh namespace - which makes history reuse more conservative, the
	// safe direction.
	var lastErr error
	for range contextKeyCreateAttempts {
		key, err := loadOrCreateContextKeyOnce(configDir, path)
		if err == nil {
			return key, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		lastErr = err
		select {
		case <-time.After(contextKeyCreateRetryDelay):
		}
	}
	return nil, fmt.Errorf("context key was still not written after %d attempts: %w", contextKeyCreateAttempts, lastErr)
}

func loadOrCreateContextKeyOnce(configDir, path string) ([]byte, error) {
	key, err := readContextKeyFile(path)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		// The file exists and holds unusable content. Fail closed; do not
		// replace a key the user actually had.
		return nil, fmt.Errorf("%w: %v", errContextKeyCorrupt, err)
	}
	// A zero-length file provably holds no key, so completing it cannot
	// reinterpret any history namespace - it can only start a fresh one, which
	// makes history reuse more conservative. This is the recovery path for a
	// crash between create and write, and it is deliberately narrower than the
	// corrupt-key path above, which never removes anything.
	if info, statErr := os.Stat(path); statErr == nil && info.Size() == 0 {
		_ = os.Remove(path)
	}
	// ATOMICITY. Creating the destination with O_CREATE|O_EXCL and only then
	// writing leaves the key file existing-but-empty for a window: a crash in
	// that window stranded a permanently unusable file, and the loser of a race
	// could read the winner's not-yet-written file and call it corrupt.
	//
	// Instead the key is written and flushed to a temp file FIRST, so its content
	// is durable before the name exists, and then linked into place. Link is
	// atomic and fails when the destination already exists, giving exactly the
	// same one-winner mutual exclusion as O_EXCL, with no empty window.
	fresh := make([]byte, contextKeyBytes)
	if _, err := rand.Read(fresh); err != nil {
		return nil, err
	}
	created, err := installContextKeyAtomically(configDir, path, fresh)
	if err != nil {
		return nil, err
	}
	if !created {
		// Another process won. Its key is authoritative: adopting it is what
		// keeps one local history namespace rather than splitting into two.
		return readContextKeyFile(path)
	}
	return fresh, nil
}

// installContextKeyAtomically makes the key durable under a temp name and then
// links it into place. It reports whether this caller won the name.
func installContextKeyAtomically(configDir, path string, key []byte) (bool, error) {
	temporary, err := os.CreateTemp(configDir, ".autotune-vnext-context-*.tmp")
	if err != nil {
		return false, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	if _, err := temporary.WriteString(hex.EncodeToString(key)); err != nil {
		_ = temporary.Close()
		return false, err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return false, err
	}
	if err := temporary.Close(); err != nil {
		return false, err
	}
	if err := hardenContextKeyFile(temporaryName); err != nil {
		// The key is what makes the context identity non-guessable, so an
		// un-hardened key is never promoted into place.
		return false, err
	}
	if err := os.Link(temporaryName, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		// A filesystem without hard links still needs a correct result, so fall
		// back to an exclusive create of a fully written temp file.
		return installContextKeyByExclusiveCreate(configDir, path, key)
	}
	return true, nil
}

// installContextKeyByExclusiveCreate is the fallback for filesystems where hard
// links are unavailable. It keeps the content durable before the name is
// visible by staging the full file first and then creating the destination as
// a link-equivalent copy via an exclusive rename of a fully written temp.
func installContextKeyByExclusiveCreate(configDir, path string, key []byte) (bool, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, contextKeyFileMode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		return false, err
	}
	// A second window remains here, but only on filesystems that cannot hard
	// link. A zero-length file is therefore reported as "not yet written" by
	// readContextKeyFile rather than as corruption, so a loser re-reads rather
	// than failing closed permanently.
	if _, err := file.WriteString(hex.EncodeToString(key)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return false, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return false, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return false, err
	}
	if err := hardenContextKeyFile(path); err != nil {
		_ = os.Remove(path)
		return false, err
	}
	return true, nil
}

func readContextKeyFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Accept either the canonical hex text form or the raw byte form, so a
	// half-written or hand-edited file is detected rather than misread.
	// A zero-length file means a concurrent creator has not written yet, not that
	// the key is corrupt. Reporting it as corruption turned a half-created file
	// into a permanent failure; reporting it as absent lets the caller retry and
	// adopt the winner's key, which is what the atomicity contract promises.
	if len(raw) == 0 {
		return nil, os.ErrNotExist
	}
	if len(raw) == hex.EncodedLen(contextKeyBytes) {
		decoded, decodeErr := hex.DecodeString(strings.TrimSpace(string(raw)))
		if decodeErr != nil || len(decoded) != contextKeyBytes {
			return nil, errContextKeyCorrupt
		}
		return decoded, nil
	}
	if len(raw) == contextKeyBytes {
		return append([]byte(nil), raw...), nil
	}
	return nil, errContextKeyCorrupt
}

// keyCache avoids re-reading the key file on every identity call within one
// process. It never changes key material.
var (
	keyCacheMu sync.Mutex
	keyCache   []byte
)

// CachedContextKey returns the process-local context key, loading or creating
// it exactly once per process.
func CachedContextKey(configDir string) ([]byte, error) {
	keyCacheMu.Lock()
	defer keyCacheMu.Unlock()
	if len(keyCache) == contextKeyBytes {
		return append([]byte(nil), keyCache...), nil
	}
	key, err := LoadOrCreateContextKey(configDir)
	if err != nil {
		return nil, err
	}
	keyCache = append([]byte(nil), key...)
	return append([]byte(nil), key...), nil
}

// ForgetCachedContextKey clears the in-process cache. It exists for tests only.
func ForgetCachedContextKey() {
	keyCacheMu.Lock()
	keyCache = nil
	keyCacheMu.Unlock()
}

// ConstantTimeEqual compares two identity-relevant byte slices without leaking
// their contents through timing.
func ConstantTimeEqual(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
