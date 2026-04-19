package storage

import "fmt"

const (
	GroupingsVersion1HeadersSize = 32

	// Versioning properties
	//
	GroupingsVersionHeaderOffset = 0

	GroupingsVersion1 = 1
)

type Groupings struct {
	// Headers - binary headers of the groupings.
	Headers []byte
	// Data - binary data of the groupings.
	Data [][]byte
}

// GroupingsNew - creates a new Groupings instance.
func GroupingsNew(headers []byte, data [][]byte) *Groupings {
	return &Groupings{
		Headers: headers,
		Data:    data,
	}
}

// Version - returns the version of the groupings.
func (g *Groupings) Version() byte {
	return g.Headers[GroupingsVersionHeaderOffset]
}

// VerifyHeaders - verifies the validity of the headers.
func (g *Groupings) VerifyHeaders() error {

	// First, determine the length of the headers. This is
	// done by summing the offset of all the headers.
	//
	headersMaxOffset := GroupingsVersionHeaderOffset + 1
	if len(g.Headers) < headersMaxOffset {
		// When the headers length does not match
		// the expected length, the headers
		// are considered invalid.
		return fmt.Errorf(
			"invalid headers: expected length at least %d, got %d",
			headersMaxOffset,
			len(g.Headers),
		)
	}

	// Next, ensure that the version of the groupings is supported.
	//
	if g.Version() != GroupingsVersion1 {
		return fmt.Errorf(
			"unsupported groupings version: got %d",
			g.Version(),
		)
	}

	// Next, perform byte padding check to ensure that headers
	// are properly padded to a byte boundary.
	//
	switch g.Version() {
	case GroupingsVersion1:
		if len(g.Headers) < GroupingsVersion1HeadersSize {
			return fmt.Errorf(
				"invalid headers: expected length at least %d, got %d",
				GroupingsVersion1HeadersSize,
				len(g.Headers),
			)
		}
	}

	return nil
}
