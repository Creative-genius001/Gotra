// Command agent is the Gotra CLI agent that exposes a local port through a
// gateway tunnel.
//
// Usage:
//
//	gotra http <port> --token <jwt> --project <id> [--gateway ws://host/ws/agent]
//	gotra version
//
// Token, project and gateway may also be supplied via GOTRA_TOKEN,
// GOTRA_PROJECT and GOTRA_GATEWAY environment variables.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/gotra/gotra/internal/agent"
)

const agentVersion = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("gotra agent %s\n", agentVersion)
	case "http":
		runHTTP(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func runHTTP(args []string) {
	// Pull the first bare (non-flag) argument out as the port so flags may
	// appear either before or after it (e.g. "gotra http 9999 --token ...").
	portArg, rest := extractPositional(args)

	fs := flag.NewFlagSet("http", flag.ExitOnError)
	token := fs.String("token", os.Getenv("GOTRA_TOKEN"), "user access token (or GOTRA_TOKEN)")
	project := fs.String("project", os.Getenv("GOTRA_PROJECT"), "project id (or GOTRA_PROJECT)")
	gatewayURL := fs.String("gateway", envOr("GOTRA_GATEWAY", "ws://localhost:8081/ws/agent"), "gateway WebSocket URL")
	_ = fs.Parse(rest)

	if portArg == "" {
		fmt.Println("error: missing local port\n\nusage: gotra http <port> --token <jwt> --project <id>")
		os.Exit(1)
	}
	port, err := strconv.Atoi(portArg)
	if err != nil || port <= 0 || port > 65535 {
		fmt.Printf("error: invalid port %q\n", portArg)
		os.Exit(1)
	}
	if *token == "" || *project == "" {
		fmt.Println("error: --token and --project are required (or set GOTRA_TOKEN / GOTRA_PROJECT)")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("gotra agent %s — connecting to %s\n", agentVersion, *gatewayURL)
	err = agent.Run(ctx, agent.Config{
		GatewayURL: *gatewayURL,
		Token:      *token,
		ProjectID:  *project,
		LocalPort:  port,
	})
	if err != nil && ctx.Err() == nil {
		fmt.Printf("gotra: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Printf(`gotra agent %s

usage:
  gotra http <port> --token <jwt> --project <id> [--gateway ws://host/ws/agent]
  gotra version

environment:
  GOTRA_TOKEN, GOTRA_PROJECT, GOTRA_GATEWAY
`, agentVersion)
}

// extractPositional returns the first non-flag argument and the remaining args
// (with that argument removed), so flags may be placed on either side of it.
func extractPositional(args []string) (string, []string) {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			rest := make([]string, 0, len(args)-1)
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+1:]...)
			return a, rest
		}
	}
	return "", args
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
