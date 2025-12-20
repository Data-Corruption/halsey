package xcrypto

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// FileSHA256 returns the SHA256 hash of the given file.
func FileSHA256(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// IsSHA256LowerHex reports whether s is exactly a SHA-256 hex digest:
// 64 chars, all in [0-9a-f].
func IsSHA256LowerHex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < 64; i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}
