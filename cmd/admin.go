package main

import (
	"log"
	"os"
	"os/exec"
)

// isAdmin checks if the current session has administrative privileges.
func isAdmin() bool {
	cmd := exec.Command("net", "session")
	return cmd.Run() == nil
}

// ensureAdmin restarts the program with elevated privileges if necessary.
func ensureAdmin() {
	if !isAdmin() {
		log.Println("Not running as administrator. Restarting with elevated privileges...")
		exePath, err := os.Executable()
		if err != nil {
			log.Fatalf("Failed to get exe path: %v", err)
		}
		cmd := exec.Command("powershell", "-Command", "Start-Process", exePath, "-Verb", "runas")
		if err := cmd.Start(); err != nil {
			log.Fatalf("Failed to elevate privileges: %v", err)
		}
		os.Exit(0)
	}
}
