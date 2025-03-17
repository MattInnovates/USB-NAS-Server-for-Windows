package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/StackExchange/wmi"
	"github.com/eiannone/keyboard"
)

// --------------- UPDATE CONFIG ---------------

// Current version of your tool; this will be overwritten at build time via ldflags.
var currentVersion = "1.0.0"

// URL to a text file containing the latest version (e.g., "1.0.1")
var versionFileURL = "http://86.129.89.196/USB-NAS/latestVersion.txt"

// --------------- AUTO-UPDATE LOGIC ---------------

// checkAndUpdate compares currentVersion with the server's latest version.
// If an update is available, it prompts the user and, if confirmed, downloads
// and launches an updater process to replace the running exe.
func checkAndUpdate() {
	remoteVersionStr, err := fetchLatestVersion()
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
			if err := performUpdate(); err != nil {
				log.Fatalf("Update failed: %v", err)
			}
			// performUpdate will launch the updater and exit.
		}
	} else {
		log.Printf("No updates found. Current version %s is up-to-date.", currentVersion)
	}
}

// fetchLatestVersion retrieves the version string from versionFileURL.
func fetchLatestVersion() (string, error) {
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
	remoteVersion := strings.TrimSpace(string(buf[:n]))
	return remoteVersion, nil
}

// performUpdate downloads the new binary to a temporary file, fetches the remote version,
// and launches the updater process. The updater will rename the new binary to include the version.
func performUpdate() error {
	// Fetch the remote version.
	remoteVersion, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to fetch remote version during update: %v", err)
	}

	// Build the download URL using the remote version.
	downloadURL := fmt.Sprintf("http://86.129.89.196/USB-NAS/usb-nas-cli-v%s.exe", remoteVersion)
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

	// Write the new binary to a temporary file.
	tmpFile := exePath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("could not create temp file: %v", err)
	}
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		out.Close()
		return fmt.Errorf("failed to write to temp file: %v", err)
	}
	out.Close()

	// Launch updater mode. Pass the temp file path, current exe path, and remote version.
	updaterCmd := exec.Command(exePath, "--update", tmpFile, exePath, remoteVersion)
	if err := updaterCmd.Start(); err != nil {
		return fmt.Errorf("failed to launch updater: %v", err)
	}
	os.Exit(0)
	return nil
}

// updaterMain is invoked when the program is run with the "--update" flag.
// It waits for the original process to exit, removes the old exe,
// renames the temp file to include the version (e.g., usb-nas-cli-v1.0.1.exe),
// and launches the new version.
func updaterMain() {
	if len(os.Args) < 5 {
		fmt.Println("Updater mode: not enough arguments.")
		os.Exit(1)
	}
	tmpFile := os.Args[2]
	exePath := os.Args[3]
	remoteVersion := os.Args[4]

	// Build new exe name: e.g., "usb-nas-cli-v1.0.1.exe"
	newExeName := fmt.Sprintf("usb-nas-cli-v%s.exe", remoteVersion)
	newExeFullPath := filepath.Join(filepath.Dir(exePath), newExeName)

	fmt.Println("Updater waiting for the original process to exit...")
	time.Sleep(5 * time.Second)

	// Remove the old exe.
	if err := os.Remove(exePath); err != nil {
		fmt.Printf("Updater failed to remove old exe: %v\n", err)
		os.Exit(1)
	}

	// Rename the temp file to the new exe name.
	if err := os.Rename(tmpFile, newExeFullPath); err != nil {
		fmt.Printf("Updater failed to rename temp exe: %v\n", err)
		os.Exit(1)
	}

	// Launch the new version.
	cmd := exec.Command(newExeFullPath)
	if err := cmd.Start(); err != nil {
		fmt.Printf("Updater failed to launch new version: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Updater replaced the exe successfully as %s.\n", newExeFullPath)
	os.Exit(0)
}

// --------------- SMB & TEMP USER CONFIG ---------------

// Win32_LogicalDisk represents the structure for querying disk information from WMI.
type Win32_LogicalDisk struct {
	DeviceID   string // e.g., "C:"
	VolumeName string // e.g., "System"
	DriveType  uint32 // 2: Removable, 3: Local
}

// getDrives queries WMI for all logical drives of type removable or local.
func getDrives() ([]Win32_LogicalDisk, error) {
	var drives []Win32_LogicalDisk
	err := wmi.Query("SELECT DeviceID, VolumeName, DriveType FROM Win32_LogicalDisk WHERE DriveType=2 OR DriveType=3", &drives)
	return drives, err
}

// driveTypeDesc returns a human-readable description of the drive type.
func driveTypeDesc(driveType uint32) string {
	switch driveType {
	case 2:
		return "Removable (USB/SD)"
	case 3:
		return "Local Disk"
	default:
		return "Unknown"
	}
}

// getMainIP returns the primary IP by "dialing" an external address (e.g. 8.8.8.8).
func getMainIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Printf("Error determining main IP: %v. Defaulting to localhost", err)
		return "localhost"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	log.Printf("Main IP detected: %s", localAddr.IP.String())
	return localAddr.IP.String()
}

// createTempUser creates a temporary local user with a given username and password.
func createTempUser(username, password string) error {
	log.Printf("Creating temp user: %s", username)
	cmd := exec.Command("net", "user", username, password, "/add")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create user %s: %v\nCommand Output:\n%s", username, err, string(output))
	}

	// Add the user to the "Users" group.
	cmd = exec.Command("net", "localgroup", "Users", username, "/add")
	groupOutput, groupErr := cmd.CombinedOutput()
	if groupErr != nil {
		if !strings.Contains(string(groupOutput), "1378") { // 1378 means already a member.
			return fmt.Errorf("failed to add user %s to 'Users' group: %v\nCommand Output:\n%s", username, groupErr, string(groupOutput))
		}
		log.Printf("User %s is already in 'Users' group.", username)
	}

	return nil
}

// deleteTempUser removes the local user created earlier.
func deleteTempUser(username string) error {
	log.Printf("Deleting temp user: %s", username)
	cmd := exec.Command("net", "user", username, "/delete")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete user %s: %v", username, err)
	}
	return nil
}

// createSMBShare creates an SMB share for the specified drive using PowerShell,
// granting FullAccess to the temporary user only.
func createSMBShare(shareName, drivePath, tempUser string) error {
	log.Printf("Attempting to create SMB share: Name=%s, Path=%s, FullAccess=%s", shareName, drivePath, tempUser)
	cmd := exec.Command("powershell", "New-SmbShare", "-Name", shareName, "-Path", drivePath, "-FullAccess", tempUser)
	return cmd.Run()
}

// removeSMBShare removes an existing SMB share by name using PowerShell.
func removeSMBShare(shareName string) error {
	log.Printf("Attempting to remove SMB share: %s", shareName)
	cmd := exec.Command("powershell", "Remove-SmbShare", "-Name", shareName, "-Force")
	return cmd.Run()
}

// isAdmin checks if the program is running with administrative privileges.
func isAdmin() bool {
	cmd := exec.Command("net", "session")
	err := cmd.Run()
	return err == nil
}

// ensureAdmin restarts the program with elevated privileges if not already admin.
func ensureAdmin() {
	if !isAdmin() {
		log.Println("Not running as administrator. Attempting to restart with elevated privileges...")
		exePath, err := os.Executable()
		if err != nil {
			log.Fatalf("Failed to get executable path: %v", err)
		}
		cmd := exec.Command("powershell", "-Command", "Start-Process", exePath, "-Verb", "runas")
		if err := cmd.Start(); err != nil {
			log.Fatalf("Failed to elevate privileges: %v", err)
		}
		os.Exit(0)
	}
}

// --------------- MAIN ---------------

func main() {
	// If run with "--update" flag, run updater mode.
	if len(os.Args) > 1 && os.Args[1] == "--update" {
		updaterMain()
	}

	// Set up logging to file usb-nas-cli.log.
	logFile, err := os.OpenFile("usb-nas-cli.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("usb-nas-cli v%s started.", currentVersion)

	// Ensure admin privileges.
	ensureAdmin()

	// Check for updates before proceeding.
	checkAndUpdate()

	// Proceed with SMB share functionality.
	for {
		log.Println("Detecting drives...")
		fmt.Println("Detecting all local and USB drives...")

		drives, err := getDrives()
		if err != nil {
			log.Fatalf("Error detecting drives: %v", err)
		}

		if len(drives) == 0 {
			fmt.Println("No drives detected.")
			log.Println("No drives detected. Exiting.")
			return
		}

		fmt.Println("Drives Found:")
		for i, drive := range drives {
			fmt.Printf("[%d] %s (%s) - %s\n", i+1, drive.DeviceID, drive.VolumeName, driveTypeDesc(drive.DriveType))
		}

		var choice int
		fmt.Print("\nEnter the number of the drive you want to share: ")
		_, err = fmt.Scan(&choice)
		if err != nil || choice < 1 || choice > len(drives) {
			log.Fatalf("Invalid drive selection: %v", err)
		}
		log.Printf("Drive selected: %d", choice)

		selectedIP := getMainIP()
		selectedDrive := drives[choice-1]
		shareName := strings.Trim(selectedDrive.DeviceID, ":\\")
		drivePath := selectedDrive.DeviceID + "\\"

		// Generate a short, realistic user/pass.
		num, err := rand.Int(rand.Reader, big.NewInt(90))
		if err != nil {
			log.Fatalf("Failed to generate random number: %v", err)
		}
		randomNumber := num.Int64() + 10
		tempUser := fmt.Sprintf("smbuser%d", randomNumber)
		tempPass := fmt.Sprintf("SmbPass!%d", randomNumber)

		// Create temp user.
		if err := createTempUser(tempUser, tempPass); err != nil {
			log.Fatalf("Error creating temp user: %v", err)
		}

		fmt.Printf("\nCreating SMB share '%s' for drive '%s'...\n", shareName, drivePath)
		log.Printf("Creating SMB share: Name=%s, DrivePath=%s, Using IP=%s, TempUser=%s", shareName, drivePath, selectedIP, tempUser)

		if err := createSMBShare(shareName, drivePath, tempUser); err != nil {
			_ = deleteTempUser(tempUser) // clean up user
			log.Fatalf("Error creating SMB share: %v", err)
		}

		fmt.Printf("SMB Share '%s' created successfully!\n", shareName)
		fmt.Printf("Access it via \\\\%s\\%s\n", selectedIP, shareName)
		fmt.Printf("Use credentials:\n  Username: %s\n  Password: %s\n", tempUser, tempPass)

		fmt.Println("Press Ctrl+K to stop sharing...")
		log.Printf("SMB Share '%s' created. Awaiting Ctrl+K to remove share.", shareName)

		if err := keyboard.Open(); err != nil {
			log.Fatalf("Error opening keyboard: %v", err)
		}
		defer keyboard.Close()

		for {
			_, key, err := keyboard.GetKey()
			if err != nil {
				log.Fatalf("Error reading keyboard input: %v", err)
			}
			if key == keyboard.KeyCtrlK {
				fmt.Println("\nStopping SMB share...")
				log.Printf("Ctrl+K detected. Stopping SMB share '%s'", shareName)

				if err := removeSMBShare(shareName); err != nil {
					log.Fatalf("Error removing SMB share: %v", err)
				}

				if err := deleteTempUser(tempUser); err != nil {
					log.Printf("Error deleting temp user: %v", err)
				}

				fmt.Println("SMB share stopped successfully.\n")
				log.Printf("SMB share '%s' removed successfully.", shareName)
				break
			}
		}
	}
}
