package runners

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Step represents a single executable step within a pipeline.
type Step struct {
	// Name is a human-readable label for the step.
	Name string
	// Command is the program to run (resolved via PATH, OS-independent).
	Command string
	// Args are the arguments passed to the command.
	Args []string
	// Dir is the optional working directory. Empty means current directory.
	Dir string
	// Env holds additional environment variables in "KEY=VALUE" format.
	Env []string
	// Emit lists events to fire after this step completes successfully.
	Emit []EmitDef
}

// EmitDef - Describes an event that a step or pipeline can fire.
type EmitDef struct {
	// Event is the trigger name for the emitted event.
	Event string
	// Pipeline is the target pipeline name to trigger.
	Pipeline string
}

// Result holds the output of a completed step.
type Result struct {
	Stdout string
	Stderr string
}

// Execute - Runs the step's command respecting the context for cancellation and timeout.
// Returns a Result containing captured stdout and stderr, and an error if the command failed.
func (s *Step) Execute(ctx context.Context) (Result, error) {
	fullCommand := s.Command
	if len(s.Args) > 0 {
		fullCommand += " " + strings.Join(s.Args, " ")
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", fullCommand)

	if s.Dir != "" {
		cmd.Dir = s.Dir
	}
	if len(s.Env) > 0 {
		cmd.Env = append(cmd.Environ(), s.Env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Result{
			Stdout: strings.TrimSpace(stdout.String()),
			Stderr: strings.TrimSpace(stderr.String()),
		}, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return Result{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}, nil
}

