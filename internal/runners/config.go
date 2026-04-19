package runners

import "time"

// Config holds runner configuration, mapped from the YAML runners section.
type Config struct {
	Timeout       time.Duration
	MaxConcurrent int
}
