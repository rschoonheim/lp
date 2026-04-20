#!/bin/bash

set -e

mkdir -p .binaries

# Build binary for windows
#
GOOS=windows GOARCH=amd64 go build -o .binaries/lp-windows.exe ./cmd/server
