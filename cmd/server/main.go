package main

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	cfg, err := loadConfiguration()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	s := newServer(cfg)
	s.registerCLIHooks()
	s.registerRunTracking()
	s.registerWSBroadcast()
	s.registerObservability()
	s.printStartupSummary()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.run(ctx, cancel)
}

// loadConfiguration - Reads and parses the YAML configuration file from os.Args.
func loadConfiguration() (Configuration, error) {
	if len(os.Args) < 2 {
		return Configuration{}, fmt.Errorf("Usage: lp-server <configuration-yaml-file>")
	}

	path := os.Args[1]
	data, err := os.ReadFile(path)
	if err != nil {
		return Configuration{}, fmt.Errorf("reading configuration file: %w", err)
	}

	var cfg Configuration
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Configuration{}, fmt.Errorf("parsing configuration file: %w", err)
	}

	return cfg, nil
}
