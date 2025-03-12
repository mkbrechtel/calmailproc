package vdir

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashFilename generates a SHA256 hash of the input string and returns it as a hex string
// This is used to create safe filenames from event UIDs
func HashFilename(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))
}