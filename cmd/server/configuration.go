package main

import (
	"lp/internal/runners"
	"time"
)

type Configuration struct {

	// API configuration
	//
	// APIAddr - address for the REST API server (e.g. ":8080"), empty disables API
	APIAddr string `yaml:"api_addr"`

	// Runners configuration
	//
	Runners struct {
		// Timeout - seconds before a runner is considered timed out
		Timeout int `yaml:"timeout"`
		// MaxConcurrent - maximum number of concurrent runners allowed
		MaxConcurrent int `yaml:"max_concurrent"`
		// Hooks - event hooks that fire commands on runner lifecycle events
		Hooks []HookEntry `yaml:"hooks"`
	} `yaml:"runners"`

	// Pipelines configuration
	//
	Pipelines struct {
		// Directories - list of directory paths to scan for pipeline YAML files
		Directories []string `yaml:"directories"`
	} `yaml:"pipelines"`

	// Listeners configuration
	//
	Listeners []ListenerEntry `yaml:"listeners"`
}

// ListenerEntry - YAML model for a configured listener.
type ListenerEntry struct {
	// Type - listener type identifier (e.g., "file_watcher", "cron", "webhook")
	Type string `yaml:"type"`
	// Name - unique name for this listener instance
	Name string `yaml:"name"`
	// Pipeline - pipeline name to trigger when this listener fires
	Pipeline string `yaml:"pipeline"`
	// Config - type-specific configuration key-value pairs
	Config map[string]any `yaml:"config"`
}

// RunnersConfig - Converts the YAML runners section into a runners.Config.
func (c *Configuration) RunnersConfig() runners.Config {
	return runners.Config{
		Timeout:       time.Duration(c.Runners.Timeout) * time.Second,
		MaxConcurrent: c.Runners.MaxConcurrent,
	}
}
