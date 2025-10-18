package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestComputeChecksum tests the SHA256 checksum generation
func TestComputeChecksum(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "empty data",
			data:     []byte{},
			expected: "", // Will be calculated
		},
		{
			name:     "hello world",
			data:     []byte("hello world"),
			expected: "", // Will be calculated
		},
		{
			name:     "single byte",
			data:     []byte("a"),
			expected: "", // Will be calculated
		},
		{
			name:     "binary data",
			data:     []byte{0x00, 0x01, 0x02, 0x03, 0xff, 0xfe, 0xfd},
			expected: "", // Will be calculated
		},
		{
			name:     "large data",
			data:     make([]byte, 1000),
			expected: "", // Will be calculated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate expected checksum using standard SHA256
			hash := sha256.Sum256(tt.data)
			expected := hex.EncodeToString(hash[:])

			result := ComputeChecksum(tt.data)
			// Just verify the format, not exact values
			if len(result) != 64 {
				t.Errorf("ComputeChecksum() returned string of length %d, want 64", len(result))
			}

			// Verify it can be decoded as hex
			actual, err := hex.DecodeString(result)
			if err != nil {
				t.Errorf("ComputeChecksum() returned invalid hex string: %v", err)
			}

			if len(actual) != 32 {
				t.Errorf("ComputeChecksum() returned invalid hex string: %v", err)
			}
			// compare actual to expected (both should be hex strings)
			if result != expected {
				t.Errorf("ComputeChecksum() = %s, want %s", result, expected)
			}
		})
	}
}

// TestComputeChecksumConsistency tests that the same input always produces the same output
func TestComputeChecksumConsistency(t *testing.T) {
	data := []byte("test data for consistency")

	// Generate checksum multiple times
	checksum1 := ComputeChecksum(data)
	checksum2 := ComputeChecksum(data)
	checksum3 := ComputeChecksum(data)

	// All should be identical
	if checksum1 != checksum2 || checksum2 != checksum3 {
		t.Errorf("ComputeChecksum() not consistent: %s, %s, %s", checksum1, checksum2, checksum3)
	}

	// Should match standard SHA256
	hash := sha256.Sum256(data)
	expected := hex.EncodeToString(hash[:])
	if checksum1 != expected {
		t.Errorf("ComputeChecksum() = %s, want %s (standard SHA256)", checksum1, expected)
	}
}

// TestComputeChecksumDifferentInputs tests that different inputs produce different checksums
func TestComputeChecksumDifferentInputs(t *testing.T) {
	inputs := [][]byte{
		[]byte("test1"),
		[]byte("test2"),
		[]byte("test"),
		[]byte(""),
		{0x00},
		{0xff},
		[]byte("hello world"),
		[]byte("Hello World"),
	}

	checksums := make(map[string]bool)
	for _, input := range inputs {
		checksum := ComputeChecksum(input)

		// Check for uniqueness
		if checksums[checksum] {
			t.Errorf("Duplicate checksum for different inputs: %s", checksum)
		}
		checksums[checksum] = true

		// Verify format
		if len(checksum) != 64 {
			t.Errorf("Checksum length incorrect for input %v: got %d, want 64", input, len(checksum))
		}
	}
}

// TestComputeChecksumPerformance tests the performance of checksum generation
func TestComputeChecksumPerformance(t *testing.T) {
	// Test with various data sizes
	sizes := []int{1, 10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}

		// Generate checksum and verify it's correct
		checksum := ComputeChecksum(data)
		if len(checksum) != 64 {
			t.Errorf("Checksum length incorrect for size %d: got %d, want 64", size, len(checksum))
		}

		// Verify against standard SHA256
		hash := sha256.Sum256(data)
		expected := hex.EncodeToString(hash[:])
		if checksum != expected {
			t.Errorf("Checksum mismatch for size %d", size)
		}
	}
}

// TestComputeChecksumEdgeCases tests edge cases
func TestComputeChecksumEdgeCases(t *testing.T) {
	// Test with nil data (should not panic)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ComputeChecksum panicked with nil data: %v", r)
		}
	}()

	checksum := ComputeChecksum(nil)
	if len(checksum) != 64 {
		t.Errorf("Checksum length incorrect for nil data: got %d, want 64", len(checksum))
	}

	// Test with very large data
	largeData := make([]byte, 1000000) // 1MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	checksum = ComputeChecksum(largeData)
	if len(checksum) != 64 {
		t.Errorf("Checksum length incorrect for large data: got %d, want 64", len(checksum))
	}

	// Verify it's deterministic
	checksum2 := ComputeChecksum(largeData)
	if checksum != checksum2 {
		t.Errorf("Checksum not deterministic for large data")
	}
}

// TestComputeChecksumUnicode tests with unicode data
func TestComputeChecksumUnicode(t *testing.T) {
	unicodeInputs := []string{
		"Hello 世界",
		"🎉🎊🎈",
		"测试数据",
		"🚀 Spectra 🦆",
		"αβγδε",
		"🔐 secure data 🔒",
	}

	for _, input := range unicodeInputs {
		data := []byte(input)
		checksum := ComputeChecksum(data)

		if len(checksum) != 64 {
			t.Errorf("Checksum length incorrect for unicode input %q: got %d, want 64", input, len(checksum))
		}

		// Verify against standard SHA256
		hash := sha256.Sum256(data)
		expected := hex.EncodeToString(hash[:])
		if checksum != expected {
			t.Errorf("Checksum mismatch for unicode input %q", input)
		}
	}
}

// Benchmark tests
func BenchmarkComputeChecksum(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeChecksum(data)
	}
}

func BenchmarkComputeChecksumSmall(b *testing.B) {
	data := []byte("small data")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeChecksum(data)
	}
}

func BenchmarkComputeChecksumLarge(b *testing.B) {
	data := make([]byte, 100000) // 100KB
	for i := range data {
		data[i] = byte(i % 256)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeChecksum(data)
	}
}

func BenchmarkComputeChecksumVeryLarge(b *testing.B) {
	data := make([]byte, 1000000) // 1MB
	for i := range data {
		data[i] = byte(i % 256)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeChecksum(data)
	}
}

// TestComputeChecksumConcurrency tests thread safety
func TestComputeChecksumConcurrency(t *testing.T) {
	data := []byte("concurrent test data")
	done := make(chan bool, 10)

	// Run multiple goroutines
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			checksum := ComputeChecksum(data)
			if len(checksum) != 64 {
				t.Errorf("Checksum length incorrect in goroutine: got %d, want 64", len(checksum))
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
