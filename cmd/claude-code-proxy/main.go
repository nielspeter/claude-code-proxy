package main

import (
	"fmt"
	"os"

	"github.com/claude-code-proxy/proxy/internal/config"
	"github.com/claude-code-proxy/proxy/internal/daemon"
	"github.com/claude-code-proxy/proxy/internal/server"
)

func main() {
	// Parse command and flags
	debug := false
	simpleLog := false
	command := ""

	if len(os.Args) > 1 {
		for i := 1; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch arg {
			case "-d", "--debug":
				debug = true
			case "-s", "--simple":
				simpleLog = true
			case "stop", "status", "version", "help", "-h", "--help":
				command = arg
			}
		}

		// Handle commands
		switch command {
		case "stop":
			daemon.Stop()
			return
		case "status":
			daemon.Status()
			return
		case "version":
			fmt.Println("claude-code-proxy v1.0.0")
			return
		case "help", "-h", "--help":
			printHelp()
			return
		}
	}

	// Load configuration with debug mode
	var cfg *config.Config
	var err error
	if debug {
		cfg, err = config.LoadWithDebug(true)
		fmt.Println("🐛 Debug mode enabled - full request/response logging active")
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Enable simple logging if requested
	if simpleLog {
		cfg.SimpleLog = true
		fmt.Println("📊 Simple log mode enabled - one-line summaries per request")
	}

	// Check if already running
	if daemon.IsRunning() {
		fmt.Println("Proxy is already running")
		os.Exit(0)
	}

	// Daemonize (run in background)
	if err := daemon.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
		os.Exit(1)
	}

	// Start HTTP server (blocks)
	if err := server.Start(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`Claude Code Proxy - OpenAI API proxy for Claude Code

Usage:
  claude-code-proxy [-d|--debug] [-s|--simple]  Start the proxy daemon
  claude-code-proxy stop                        Stop the proxy daemon
  claude-code-proxy status                      Check if proxy is running
  claude-code-proxy version                     Show version
  claude-code-proxy help                        Show this help

Flags:
  -d, --debug     Enable debug mode (logs full requests/responses)
  -s, --simple    Enable simple log mode (one-line summary per request)

Configuration:
  Config file locations (checked in order):
    1. ./​.env
    2. ~/.claude/proxy.env
    3. ~/.claude-code-proxy

  Required:
    OPENAI_API_KEY         Your OpenAI API key

  Optional:
    ANTHROPIC_DEFAULT_OPUS_MODEL    Override opus routing
    ANTHROPIC_DEFAULT_SONNET_MODEL  Override sonnet routing
    ANTHROPIC_DEFAULT_HAIKU_MODEL   Override haiku routing
    OPENAI_BASE_URL                 OpenAI API base URL
    HOST                            Server host (default: 0.0.0.0)
    PORT                            Server port (default: 8082)

Examples:
  # Start proxy
  claude-code-proxy

  # Use with Claude Code (via ccp wrapper)
  ccp chat

  # Or manually
  ANTHROPIC_BASE_URL=http://localhost:8082 claude chat`)
}
