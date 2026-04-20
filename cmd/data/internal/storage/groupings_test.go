package storage_test

import (
	"encoding/binary"
	"lp/cmd/data/internal/storage"
	"os"
	"path/filepath"
	"testing"
)

func TestGroupings_VerifyHeaders(t *testing.T) {
	// Test case 1: invalid headers
	//
	t.Run("Invalid headers", func(t *testing.T) {
		groupings := storage.GroupingsNew([]byte{}, nil)
		err := groupings.VerifyHeaders()
		if err == nil {
			t.Errorf("VerifyHeaders() = nil, want error")
		}

		if err.Error() != "invalid headers: expected length at least 1, got 0" {
			t.Errorf("VerifyHeaders() error = %q, want %q", err.Error(), "invalid headers: expected length at least 1, got 0")
		}
	})

	// Test case 2: invalid version
	//
	t.Run("Invalid version", func(t *testing.T) {
		groupings := storage.GroupingsNew([]byte{0x00}, nil)
		err := groupings.VerifyHeaders()
		if err == nil {
			t.Errorf("VerifyHeaders() = nil, want error")
		}

		if err.Error() != "unsupported groupings version: got 0" {
			t.Errorf(
				"VerifyHeaders() error = %q, want %q",
				err.Error(),
				"invalid version: expected version 0, got 0",
			)
		}
	})

	// Test case 3: padding mismatch
	//
	t.Run("Padding mismatch", func(t *testing.T) {
		groupings := storage.GroupingsNew([]byte{storage.GroupingsVersion1}, nil)
		err := groupings.VerifyHeaders()
		if err == nil {
			t.Errorf("VerifyHeaders() = nil, want error")
		}

		if err.Error() != "invalid headers: expected length at least 32, got 1" {
			t.Errorf(
				"VerifyHeaders() error = %q, want %q",
				err.Error(),
				"invalid headers: expected length at least 32, got 1",
			)
		}
	})

	// Test case 4: valid headers
	//
	t.Run("Valid groupings", func(t *testing.T) {
		groupings := storage.Groupings{
			Headers: []byte{
				storage.GroupingsVersion1, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00,
			},
			Data: nil,
		}
		err := groupings.VerifyHeaders()
		if err != nil {
			t.Errorf("VerifyHeaders() error = %v, want nil", err)
		}
	})
}

func TestGroupings_WriteToFile(t *testing.T) {
	// Helper to create valid v1 headers.
	//
	validHeaders := make([]byte, storage.GroupingsVersion1HeadersSize)
	validHeaders[storage.GroupingsVersionHeaderOffset] = storage.GroupingsVersion1

	// Test case 1: invalid headers
	//
	t.Run("Invalid headers", func(t *testing.T) {
		groupings := storage.GroupingsNew([]byte{}, nil)
		err := groupings.WriteToFile(filepath.Join(t.TempDir(), "out.bin"))
		if err == nil {
			t.Errorf("WriteToFile() = nil, want error")
		}
	})

	// Test case 2: write empty data
	//
	t.Run("Write empty data", func(t *testing.T) {
		groupings := storage.GroupingsNew(validHeaders, nil)
		path := filepath.Join(t.TempDir(), "out.bin")
		err := groupings.WriteToFile(path)
		if err != nil {
			t.Errorf("WriteToFile() error = %v, want nil", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}

		if len(data) != storage.GroupingsVersion1HeadersSize {
			t.Errorf("file length = %d, want %d", len(data), storage.GroupingsVersion1HeadersSize)
		}
	})

	// Test case 3: write with data entries
	//
	t.Run("Write with data entries", func(t *testing.T) {
		entries := [][]byte{
			{0xDE, 0xAD},
			{0xBE, 0xEF, 0xCA, 0xFE},
		}
		groupings := storage.GroupingsNew(validHeaders, entries)
		path := filepath.Join(t.TempDir(), "out.bin")
		err := groupings.WriteToFile(path)
		if err != nil {
			t.Errorf("WriteToFile() error = %v, want nil", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}

		// Expected: 32 (headers) + 4+2 (entry 0) + 4+4 (entry 1) = 46
		expectedLen := storage.GroupingsVersion1HeadersSize + 4 + 2 + 4 + 4
		if len(data) != expectedLen {
			t.Errorf("file length = %d, want %d", len(data), expectedLen)
		}

		// Verify first entry length prefix is 2 (big-endian).
		offset := storage.GroupingsVersion1HeadersSize
		length := binary.BigEndian.Uint32(data[offset : offset+4])
		if length != 2 {
			t.Errorf("data[0] length = %d, want 2", length)
		}

		// Verify first entry content.
		if data[offset+4] != 0xDE || data[offset+5] != 0xAD {
			t.Errorf("data[0] content = %x, want DEAD", data[offset+4:offset+6])
		}
	})

	// Test case 4: invalid path
	//
	t.Run("Invalid path", func(t *testing.T) {
		groupings := storage.GroupingsNew(validHeaders, nil)
		err := groupings.WriteToFile("/nonexistent/dir/out.bin")
		if err == nil {
			t.Errorf("WriteToFile() = nil, want error")
		}
	})
}

func TestReadGroupingsFromFile(t *testing.T) {
	// Helper to create valid v1 headers.
	//
	validHeaders := make([]byte, storage.GroupingsVersion1HeadersSize)
	validHeaders[storage.GroupingsVersionHeaderOffset] = storage.GroupingsVersion1

	// Test case 1: file does not exist
	//
	t.Run("File does not exist", func(t *testing.T) {
		_, err := storage.ReadGroupingsFromFile(filepath.Join(t.TempDir(), "missing.bin"))
		if err == nil {
			t.Errorf("ReadGroupingsFromFile() = nil, want error")
		}
	})

	// Test case 2: empty file
	//
	t.Run("Empty file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.bin")
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		_, err := storage.ReadGroupingsFromFile(path)
		if err == nil {
			t.Errorf("ReadGroupingsFromFile() = nil, want error")
		}
	})

	// Test case 3: unsupported version
	//
	t.Run("Unsupported version", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad_version.bin")
		data := make([]byte, storage.GroupingsVersion1HeadersSize)
		data[storage.GroupingsVersionHeaderOffset] = 0xFF
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		_, err := storage.ReadGroupingsFromFile(path)
		if err == nil {
			t.Errorf("ReadGroupingsFromFile() = nil, want error")
		}

		if err.Error() != "unsupported groupings version: got 255" {
			t.Errorf("ReadGroupingsFromFile() error = %q, want %q", err.Error(), "unsupported groupings version: got 255")
		}
	})

	// Test case 4: truncated headers
	//
	t.Run("Truncated headers", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "truncated.bin")
		data := []byte{storage.GroupingsVersion1}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		_, err := storage.ReadGroupingsFromFile(path)
		if err == nil {
			t.Errorf("ReadGroupingsFromFile() = nil, want error")
		}
	})

	// Test case 5: headers only, no data
	//
	t.Run("Headers only", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "headers_only.bin")
		if err := os.WriteFile(path, validHeaders, 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		groupings, err := storage.ReadGroupingsFromFile(path)
		if err != nil {
			t.Errorf("ReadGroupingsFromFile() error = %v, want nil", err)
		}

		if len(groupings.Data) != 0 {
			t.Errorf("Data length = %d, want 0", len(groupings.Data))
		}
	})

	// Test case 6: round-trip write then read
	//
	t.Run("Round-trip", func(t *testing.T) {
		entries := [][]byte{
			{0xDE, 0xAD},
			{0xBE, 0xEF, 0xCA, 0xFE},
		}
		original := storage.GroupingsNew(validHeaders, entries)
		path := filepath.Join(t.TempDir(), "roundtrip.bin")

		if err := original.WriteToFile(path); err != nil {
			t.Fatalf("WriteToFile() error = %v", err)
		}

		restored, err := storage.ReadGroupingsFromFile(path)
		if err != nil {
			t.Fatalf("ReadGroupingsFromFile() error = %v", err)
		}

		if len(restored.Headers) != len(original.Headers) {
			t.Errorf("Headers length = %d, want %d", len(restored.Headers), len(original.Headers))
		}

		if len(restored.Data) != len(original.Data) {
			t.Fatalf("Data length = %d, want %d", len(restored.Data), len(original.Data))
		}

		for i := range original.Data {
			if len(restored.Data[i]) != len(original.Data[i]) {
				t.Errorf("Data[%d] length = %d, want %d", i, len(restored.Data[i]), len(original.Data[i]))
			}
			for j := range original.Data[i] {
				if restored.Data[i][j] != original.Data[i][j] {
					t.Errorf("Data[%d][%d] = %x, want %x", i, j, restored.Data[i][j], original.Data[i][j])
				}
			}
		}
	})

	// Test case 7: truncated data entry
	//
	t.Run("Truncated data entry", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "truncated_data.bin")
		buf := make([]byte, storage.GroupingsVersion1HeadersSize+4)
		copy(buf, validHeaders)
		// Write length prefix of 100 but no actual data bytes.
		binary.BigEndian.PutUint32(buf[storage.GroupingsVersion1HeadersSize:], 100)
		if err := os.WriteFile(path, buf, 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		_, err := storage.ReadGroupingsFromFile(path)
		if err == nil {
			t.Errorf("ReadGroupingsFromFile() = nil, want error")
		}
	})
}
