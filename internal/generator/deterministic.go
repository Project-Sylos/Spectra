package generator

// GenerateDeterministicFileData produces deterministic file data and checksum based on
// a single seed value. Every call with the same seed yields the same byte pattern and checksum.
func GenerateDeterministicFileData(baseSeed int64) ([]byte, string, error) {
	rng := NewRNG(baseSeed)
	return GenerateFileData(rng)
}
