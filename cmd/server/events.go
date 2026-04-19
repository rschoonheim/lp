package main

import (
	"context"
	"lp/internal/runners"
	"os/exec"
	"strings"
	"time"
)

// HookEntry represents a configured hook from the YAML file.
type HookEntry struct {
	On      string `yaml:"on"`
	Command string `yaml:"command"`
	Timeout int    `yaml:"timeout"` // seconds, 0 = 30s default
}

// RegisterConfiguredHooks - Wires YAML-configured hook entries to the pool's hooks.
func RegisterConfiguredHooks(pool *runners.Pool, entries []HookEntry) {
	for _, entry := range entries {
		entry := entry
		timeout := time.Duration(entry.Timeout) * time.Second
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		cmd := entry.Command

		switch strings.ToLower(entry.On) {
		case "pipeline_started":
			pool.Hooks.OnPipelineStarted(func(pipeline string) {
				go runHookCommand(cmd, timeout, "LP_PIPELINE="+pipeline)
			})
		case "pipeline_completed":
			pool.Hooks.OnPipelineCompleted(func(pipeline string, results []runners.Result) {
				go runHookCommand(cmd, timeout, "LP_PIPELINE="+pipeline)
			})
		case "pipeline_failed":
			pool.Hooks.OnPipelineFailed(func(pipeline string, err error) {
				go runHookCommand(cmd, timeout, "LP_PIPELINE="+pipeline, "LP_ERROR="+err.Error())
			})
		case "step_started":
			pool.Hooks.OnStepStarted(func(pipeline, step string) {
				go runHookCommand(cmd, timeout, "LP_PIPELINE="+pipeline, "LP_STEP="+step)
			})
		case "step_completed":
			pool.Hooks.OnStepCompleted(func(pipeline, step string, result runners.Result) {
				go runHookCommand(cmd, timeout, "LP_PIPELINE="+pipeline, "LP_STEP="+step)
			})
		case "step_failed":
			pool.Hooks.OnStepFailed(func(pipeline, step string, err error) {
				go runHookCommand(cmd, timeout, "LP_PIPELINE="+pipeline, "LP_STEP="+step, "LP_ERROR="+err.Error())
			})
		}
	}
}

// runHookCommand - Executes a hook command with the given timeout and environment variables.
func runHookCommand(command string, timeout time.Duration, envVars ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Env = append(cmd.Environ(), envVars...)
	_ = cmd.Run()
}

