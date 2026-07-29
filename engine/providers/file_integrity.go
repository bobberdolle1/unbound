package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

func verifyFileSHA256(path, expected string) error {
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("expected SHA-256 is not configured")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != strings.ToLower(expected) {
		return fmt.Errorf("SHA-256 mismatch: got %s, want %s", actual, expected)
	}
	return nil
}
