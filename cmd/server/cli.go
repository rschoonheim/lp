package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// cli - Thread-safe CLI output writer with a uniform format.
// All lines are printed as: [HH:MM:SS] prefix  message
type cli struct {
	mu sync.Mutex
}

var out = &cli{}

// ANSI color codes.
const (
	colorReset   = "\033[0m"
	colorGray    = "\033[90m"
	colorRed     = "\033[91m"
	colorGreen   = "\033[92m"
	colorYellow  = "\033[93m"
	colorBlue    = "\033[94m"
	colorMagenta = "\033[95m"
	colorCyan    = "\033[96m"
	colorWhite   = "\033[97m"
)

// colored prefixes.
var (
	prefixInfo  = colorBlue + "INFO " + colorReset
	prefixOK    = colorGreen + " OK  " + colorReset
	prefixFail  = colorRed + "FAIL " + colorReset
	prefixEvent = colorMagenta + "EVNT " + colorReset
	prefixRun   = colorCyan + " RUN " + colorReset
	prefixOut   = colorWhite + " OUT " + colorReset
)

// log - Prints a formatted line to stdout with a timestamp and prefix.
func (c *cli) log(prefix, format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ts := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "%s[%s]%s %s %s\n", colorGray, ts, colorReset, prefix, msg)
}

// info - Prints an informational message.
func (c *cli) info(format string, args ...any) {
	c.log(prefixInfo, format, args...)
}

// ok - Prints a success message.
func (c *cli) ok(format string, args ...any) {
	c.log(prefixOK, format, args...)
}

// fail - Prints a failure message.
func (c *cli) fail(format string, args ...any) {
	c.log(prefixFail, format, args...)
}

// event - Prints an event message.
func (c *cli) event(format string, args ...any) {
	c.log(prefixEvent, format, args...)
}

// run - Prints an execution message.
func (c *cli) run(format string, args ...any) {
	c.log(prefixRun, format, args...)
}

// output - Prints captured command output, indented and line-split.
func (c *cli) output(stream, text string) {
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			c.log(prefixOut, "%s: %s", stream, line)
		}
	}
}
