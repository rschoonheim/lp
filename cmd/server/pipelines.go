package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lp/internal/pipeline"

	"gopkg.in/yaml.v3"
)

// LoadPipelinesFromDirectories - Scans the given directories for .yaml/.yml files,
// parses each as a Pipeline definition, and registers them in the store.
// Returns the number of pipelines loaded and any errors encountered.
func LoadPipelinesFromDirectories(store *pipeline.Store, directories []string) (int, []error) {
	var loaded int
	var errs []error

	for _, dir := range directories {
		info, err := os.Stat(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("pipeline directory %q: %w", dir, err))
			continue
		}
		if !info.IsDir() {
			errs = append(errs, fmt.Errorf("pipeline directory %q: not a directory", dir))
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("pipeline directory %q: %w", dir, err))
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".yaml" && ext != ".yml" {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			p, err := loadPipelineFile(path)
			if err != nil {
				errs = append(errs, fmt.Errorf("pipeline file %q: %w", path, err))
				continue
			}

			store.AddPipeline(p)
			loaded++
		}
	}

	return loaded, errs
}

// loadPipelineFile - Reads and parses a single pipeline YAML file.
func loadPipelineFile(path string) (*pipeline.Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var def struct {
		Name  string             `yaml:"name"`
		Steps []pipeline.StepDef `yaml:"steps"`
	}

	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if def.Name == "" {
		return nil, fmt.Errorf("missing required field 'name'")
	}
	if len(def.Steps) == 0 {
		return nil, fmt.Errorf("pipeline %q has no steps", def.Name)
	}

	return pipeline.NewPipeline(def.Name, def.Steps), nil
}

// ReloadPipelinesFromDirectories - Reloads all pipeline YAML files and hot-swaps
// them in the store. Returns the number loaded and any errors.
func ReloadPipelinesFromDirectories(store *pipeline.Store, directories []string) (int, []error) {
	var loaded []*pipeline.Pipeline
	var errs []error

	for _, dir := range directories {
		info, err := os.Stat(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("pipeline directory %q: %w", dir, err))
			continue
		}
		if !info.IsDir() {
			errs = append(errs, fmt.Errorf("pipeline directory %q: not a directory", dir))
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("pipeline directory %q: %w", dir, err))
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".yaml" && ext != ".yml" {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			p, err := loadPipelineFile(path)
			if err != nil {
				errs = append(errs, fmt.Errorf("pipeline file %q: %w", path, err))
				continue
			}

			loaded = append(loaded, p)
		}
	}

	store.ReplacePipelines(loaded)
	return len(loaded), errs
}
