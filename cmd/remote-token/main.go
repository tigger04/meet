// ABOUTME: remote-token CLI wrapper. SSHs to a target host and runs
// ABOUTME: meet token to generate a moderator JWT URL for a given room.

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Version is the build identifier, overridden at link time.
var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-version":
			fmt.Println(Version)
			os.Exit(0)
		}
	}

	if len(os.Args) < 3 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Fprintln(os.Stderr, "Usage: meet-token <host> <room> [--name <name>] [--expiry <duration>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Generate a moderator meeting URL by SSHing to the target host and")
		fmt.Fprintln(os.Stderr, "running meet token with the host's config and secrets.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintln(os.Stderr, "  meet-token light-hugger workshop-april")
		fmt.Fprintln(os.Stderr, "  meet-token light-hugger demo --name Tigger --expiry 4h")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "The host must have meet deployed with config and secrets at the")
		fmt.Fprintln(os.Stderr, "standard paths (/srv/meet/repo/config/, /etc/meet/secrets.yaml).")
		os.Exit(2)
	}

	host := os.Args[1]
	room := os.Args[2]
	extra := os.Args[3:]

	// Build the remote command. On the server, config and secrets are at
	// known paths per the deploy convention.
	configPath := fmt.Sprintf("/srv/meet/repo/config/defaults.yaml,/srv/meet/repo/config/%s.yaml,/etc/meet/secrets.yaml", host)
	remoteArgs := fmt.Sprintf("/srv/meet/meet token --config %s --room %s", configPath, room)
	for _, a := range extra {
		remoteArgs += " " + a
	}

	cmd := exec.Command("ssh", host, remoteArgs)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
