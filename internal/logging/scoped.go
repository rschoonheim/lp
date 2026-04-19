package logging

// Scoped - A convenience writer bound to a fixed category and source.
// Use it to avoid repeating category/source on every log call.
type Scoped struct {
	ledger   *Ledger
	category Category
	source   string
}

// NewScoped - Creates a Scoped writer bound to the given ledger, category, and source.
func NewScoped(ledger *Ledger, category Category, source string) *Scoped {
	return &Scoped{ledger: ledger, category: category, source: source}
}

// Info - Appends an info-level entry with the bound category and source.
func (s *Scoped) Info(message string, fields map[string]any) {
	s.ledger.Info(s.category, s.source, message, fields)
}

// Error - Appends an error-level entry with the bound category and source.
func (s *Scoped) Error(message string, fields map[string]any) {
	s.ledger.Error(s.category, s.source, message, fields)
}

// Warn - Appends a warn-level entry with the bound category and source.
func (s *Scoped) Warn(message string, fields map[string]any) {
	s.ledger.Warn(s.category, s.source, message, fields)
}

// Debug - Appends a debug-level entry with the bound category and source.
func (s *Scoped) Debug(message string, fields map[string]any) {
	s.ledger.Debug(s.category, s.source, message, fields)
}

