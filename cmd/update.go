package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var currentVersion = "0.0.1"

// Constants for update logic.
const (
	versionFileURL = "http://127.0.0.1:8080/latestVersion.txt"
	baseURL        = "http://127.0.0.1:8080"
)

// Define the UpdateLogic type.
type UpdateLogic struct{}

// Create a global instance to be used throughout the project.
var UpdateLogicInstance = &UpdateLogic{}

// CheckAndUpdate checks for a newer version and launches the update if needed.
func (u *UpdateLogic) CheckAndUpdate() {
	remoteVersionStr, err := u.fetchLatestVersion()
	if err != nil {
		log.Printf("Could not check for updates: %v", err)
		return
	}

	localSem, err := semver.NewVersion(currentVersion)
	if err != nil {
		log.Printf("Invalid local version: %v", err)
		return
	}

	remoteSem, err := semver.NewVersion(remoteVersionStr)
	if err != nil {
		log.Printf("Invalid remote version: %v", err)
		return
	}

	if remoteSem.GreaterThan(localSem) {
		log.Printf("New version %s is available (current: %s).", remoteSem, localSem)
		fmt.Printf("A new version (%s) is available. Update now? (y/n): ", remoteSem)
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(input) == "y" {
			if err := u.downloadAndLaunch(remoteSem.String()); err != nil {
				log.Fatalf("Update failed: %v", err)
			}
			os.Exit(0)
		}
	} else {
		log.Printf("No updates found. Current version %s is up-to-date.", currentVersion)
	}
}

// fetchLatestVersion fetches the version string from the version file URL.
func (u *UpdateLogic) fetchLatestVersion() (string, error) {
	resp, err := http.Get(versionFileURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("version check failed: HTTP %d", resp.StatusCode)
	}
	buf := make([]byte, 32)
	n, err := resp.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return "", err
	}
	return strings.TrimSpace(string(buf[:n])), nil
}

// downloadAndLaunch downloads the new binary and launches it using PowerShell.
func (u *UpdateLogic) downloadAndLaunch(newVer string) error {
	newExeName := fmt.Sprintf("usb-nas-cli-v%s.exe", newVer)
	downloadURL := fmt.Sprintf("%s/%s", baseURL, newExeName)
	log.Printf("Downloading new binary from: %s", downloadURL)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download new binary: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status downloading new binary: %d", resp.StatusCode)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get current exe path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	newExeFullPath := filepath.Join(exeDir, newExeName)

	out, err := os.Create(newExeFullPath)
	if err != nil {
		return fmt.Errorf("failed to create new exe file: %v", err)
	}
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		out.Close()
		return fmt.Errorf("failed to write new exe: %v", err)
	}
	out.Close()

	cmd := exec.Command("powershell", "-Command", "Start-Process", newExeFullPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch new version: %v", err)
	}
	log.Printf("Launched new version: %s", newExeFullPath)
	return nil
}

// removeOlderExecutables removes executable files with a version lower than the current version.
func (u *UpdateLogic) removeOlderExecutables(currentVerStr string) {
	currentSem, err := semver.NewVersion(currentVerStr)
	if err != nil {
		log.Printf("removeOlderExecutables: invalid current version: %v", err)
		return
	}

	files, err := ioutil.ReadDir(".")
	if err != nil {
		log.Printf("removeOlderExecutables: failed to read dir: %v", err)
		return
	}

	for _, f := range files {
		name := f.Name()

		// Remove unversioned executable if it exists.
		if name == "usb-nas-cli.exe" {
			log.Printf("Removing unversioned executable: %s", name)
			os.Remove(name)
			continue
		}

		// Remove any versioned executables that are older than the current version.
		if strings.HasPrefix(name, "usb-nas-cli-v") && strings.HasSuffix(name, ".exe") {
			verStr := strings.TrimSuffix(strings.TrimPrefix(name, "usb-nas-cli-v"), ".exe")
			ver, err := semver.NewVersion(verStr)
			if err != nil {
				continue
			}
			if ver.LessThan(currentSem) {
				log.Printf("Removing older exe: %s", name)
				os.Remove(name)
			}
		}
	}
}
