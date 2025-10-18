package generator

import (
	"crypto/sha256"
	"fmt"
)

// ComputeChecksum computes a SHA256 checksum for the given data
func ComputeChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// GenerateFileData generates 1KB of random data and returns both the data and its checksum
// This matches the requirement for 1KB files with checksum generation
func GenerateFileData(rng *RNG) ([]byte, string) {
	// Generate 1KB (1024 bytes) of random data
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(rng.Intn(256))
	}
	
	checksum := ComputeChecksum(data)
	return data, checksum
}
