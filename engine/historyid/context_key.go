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
)

// ContextKeyFileName is the local context key file inside the owned config
// directory. It is deliberately NOT part of managed activation state: it
// represents local history namespace continuity, so Suspend, Revert and graph
// Revert must never create, rotate, or delete it.
const ContextKeyFileName = "autotune_vnext_context.key"

// contextKeyBytes is 32 bytes (256 bits) of CSPRNG output.
const contextKeyBytes = 32

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
	if key, err := readContextKeyFile(path); err == nil {
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		// The file exists but is unusable. Fail closed; do not replace it.
		return nil, fmt.Errorf("%w: %v", errContextKeyCorrupt, err)
	}

	// Exclusive create. The loser of the race re-reads the winner's file.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, contextKeyFileMode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// Another process won between our read and our create.
			return readContextKeyFile(path)
		}
		return nil, err
	}
	key := make([]byte, contextKeyBytes)
	if _, err := rand.Read(key); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if _, err := file.WriteString(hex.EncodeToString(key)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	// The key is what makes the context identity non-guessable, so an
	// un-hardened key is not silently accepted: report it instead.
	if err := hardenContextKeyFile(path); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return key, nil
}

func readContextKeyFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Accept either the canonical hex text form or the raw byte form, so a
	// half-written or hand-edited file is detected rather than misread.
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
