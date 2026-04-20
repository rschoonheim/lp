package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

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

// VersionSupported - checks if the version of the groupings is supported.
func (g *Groupings) VersionSupported() bool {
	switch g.Version() {
	default:
		return false
	case GroupingsVersion1:
		return true
	}
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
	if !g.VersionSupported() {
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

// WriteToFile - writes the groupings to a file.
func (g *Groupings) WriteToFile(path string) error {
	if err := g.VerifyHeaders(); err != nil {
		return fmt.Errorf("write groupings to %s: %w", path, err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write groupings to %s: %w", path, err)
	}
	defer f.Close()

	// Write headers (padded to fixed size based on version).
	if _, err := f.Write(g.Headers); err != nil {
		return fmt.Errorf("write groupings headers: %w", err)
	}

	// Write each data entry prefixed with its 4-byte length.
	for i, d := range g.Data {
		length := uint32(len(d))
		if err := binary.Write(f, binary.BigEndian, length); err != nil {
			return fmt.Errorf("write groupings data[%d] length: %w", i, err)
		}
		if _, err := f.Write(d); err != nil {
			return fmt.Errorf("write groupings data[%d]: %w", i, err)
		}
	}

	return nil
}

// ReadGroupingsFromFile - reads groupings from a file.
func ReadGroupingsFromFile(path string) (*Groupings, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read groupings from %s: %w", path, err)
	}
	defer f.Close()

	headers, err := readGroupingsHeaders(f)
	if err != nil {
		return nil, err
	}

	data, err := readGroupingsData(f)
	if err != nil {
		return nil, err
	}

	return GroupingsNew(headers, data), nil
}

// ------------------------------------------------------------------
//
// # Helper functions
//
// ------------------------------------------------------------------

// readGroupingsHeaders - reads and validates headers from a file.
func readGroupingsHeaders(f *os.File) ([]byte, error) {
	var version [1]byte
	if _, err := f.ReadAt(version[:], GroupingsVersionHeaderOffset); err != nil {
		return nil, fmt.Errorf("read groupings version: %w", err)
	}

	var headerSize int
	switch version[0] {
	case GroupingsVersion1:
		headerSize = GroupingsVersion1HeadersSize
	default:
		return nil, fmt.Errorf("unsupported groupings version: got %d", version[0])
	}

	headers := make([]byte, headerSize)
	if _, err := io.ReadFull(f, headers); err != nil {
		return nil, fmt.Errorf("read groupings headers: %w", err)
	}

	return headers, nil
}

// readGroupingsData - reads length-prefixed data entries from a file.
func readGroupingsData(f *os.File) ([][]byte, error) {
	var data [][]byte
	for {
		var length uint32
		if err := binary.Read(f, binary.BigEndian, &length); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read groupings data length: %w", err)
		}

		entry := make([]byte, length)
		if _, err := io.ReadFull(f, entry); err != nil {
			return nil, fmt.Errorf("read groupings data: %w", err)
		}
		data = append(data, entry)
	}

	return data, nil
}
