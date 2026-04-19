package storage

import (
	"crypto/sha256"

	"github.com/google/uuid"
)

type Grouping struct {
	Context uuid.UUID
	Type    string
}

// Identifier - returns the unique identifier of the grouping.
func (g *Grouping) Identifier() []byte {
	h := sha256.New()
	h.Write(g.Context[:])
	h.Write([]byte(g.Type))
	return h.Sum(nil)
}
