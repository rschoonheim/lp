package storage_test

import (
	"lp/cmd/data/internal/storage"
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
