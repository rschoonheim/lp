package storage_test

import (
	"fmt"
	"lp/cmd/data/internal/storage"
	"testing"

	"github.com/google/uuid"
)

func TestGrouping_Identifier(t *testing.T) {
	uuid, _ := uuid.Parse("3b079575-b0ac-44e9-973e-90f735877f17")
	grouping := storage.Grouping{
		Context: uuid,
		Type:    "test-case",
	}

	if fmt.Sprintf("%x", grouping.Identifier()) != "d785465a80ff8d3876929be63f3b688c0c7160167d0e558fe10fde6a6b918f7d" {
		t.Errorf(
			"Identifier() = %x, want %x",
			grouping.Identifier(),
			"d785465a80ff8d3876929be63f3b688c0c7160167d0e558fe10fde6a6b918f7d",
		)
	}
}
