package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/StackExchange/wmi"
	"github.com/eiannone/keyboard"
)

// ------------------ CONFIG & VERSION ------------------

// Current version (set via ldflags or manually)
var currentVersion = "1.0.0"

// URL hosting the text file with the latest version (e.g. "1.0.3")
var versionFileURL = "http://86.129.89.196/USB-NAS/latestVersion.txt"

// Base URL for the EXEs (we'll append "-vX.Y.Z.exe" at runtime)
var baseURL = "http://86.129.89.196/USB-NAS"

// ------------------ UPDATE LOGIC ------------------

// checkAndUpdate downloads the new version (if any) and launches it.
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
			if err := downloadAndLaunch(remoteSem.String()); err != nil {
				log.Fatalf("Update failed: %v", err)
			}
			os.Exit(0)
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
	return strings.TrimSpace(string(buf[:n])), nil
}

// downloadAndLaunch downloads "usb-nas-cli-v<version>.exe" and launches it.
func downloadAndLaunch(newVer string) error {
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

// removeOlderExecutables scans the current directory for older usb-nas-cli-vX.Y.Z.exe files
// and removes any version less than currentVersion.
func removeOlderExecutables(currentVerStr string) {
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

// ------------------ SMB & TEMP USER LOGIC ------------------

// Win32_LogicalDisk represents the structure for querying disk information from WMI.
type Win32_LogicalDisk struct {
	DeviceID   string // e.g., "C:"
	VolumeName string // e.g., "System"
	DriveType  uint32 // 2: Removable, 3: Local
}

// getDrivesSMB queries WMI for removable/local disks.
func getDrivesSMB() ([]Win32_LogicalDisk, error) {
	var drives []Win32_LogicalDisk
	err := wmi.Query("SELECT DeviceID, VolumeName, DriveType FROM Win32_LogicalDisk WHERE DriveType=2 OR DriveType=3", &drives)
	return drives, err
}

// driveTypeDescSMB returns a friendly drive type.
func driveTypeDescSMB(driveType uint32) string {
	switch driveType {
	case 2:
		return "Removable (USB/SD)"
	case 3:
		return "Local Disk"
	default:
		return "Unknown"
	}
}

// getMainIPSMB returns the primary IP by "dialing" 8.8.8.8.
func getMainIPSMB() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Printf("Error determining main IP: %v. Using localhost", err)
		return "localhost"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ipStr := localAddr.IP.String()
	if ipStr == "127.0.0.1" {
		return "localhost"
	}
	return ipStr
}

// createTempUser creates a temporary local user.
func createTempUser(username, password string) error {
	log.Printf("Creating temp user: %s", username)
	cmd := exec.Command("net", "user", username, password, "/add")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create user %s: %v\nOutput:\n%s", username, err, string(output))
	}
	cmd = exec.Command("net", "localgroup", "Users", username, "/add")
	groupOutput, groupErr := cmd.CombinedOutput()
	if groupErr != nil && !strings.Contains(string(groupOutput), "1378") {
		return fmt.Errorf("failed to add user %s to 'Users' group: %v\nOutput:\n%s", username, groupErr, string(groupOutput))
	}
	return nil
}

// deleteTempUser removes the local user.
func deleteTempUser(username string) error {
	log.Printf("Deleting temp user: %s", username)
	cmd := exec.Command("net", "user", username, "/delete")
	return cmd.Run()
}

// createSMBShare creates an SMB share granting access to both the temp user and Everyone.
// It first attempts to remove any existing share with the same name.
func createSMBShare(shareName, drivePath, tempUser string) error {
	// Attempt to remove any existing share with the same name.
	_ = exec.Command("powershell", "-Command", "Remove-SmbShare", "-Name", shareName, "-Force").Run()

	// Build the FullAccess list: tempUser and Everyone.
	fullAccess := fmt.Sprintf("%s,Everyone", tempUser)
	log.Printf("Creating SMB share: %s -> %s, FullAccess=%s", shareName, drivePath, fullAccess)

	// Create the new share.
	cmd := exec.Command("powershell", "-Command", "New-SmbShare", "-Name", shareName, "-Path", drivePath, "-FullAccess", fullAccess)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create SMB share: %v\nOutput: %s", err, string(out))
	}
	return nil
}

// removeSMBShare removes an SMB share.
func removeSMBShare(shareName string) error {
	log.Printf("Removing SMB share: %s", shareName)
	cmd := exec.Command("powershell", "-Command", "Remove-SmbShare", "-Name", shareName, "-Force")
	return cmd.Run()
}

// isAdmin checks if running as admin.
func isAdmin() bool {
	cmd := exec.Command("net", "session")
	return cmd.Run() == nil
}

// ensureAdmin re-launches as admin if needed.
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

// ------------------ MAIN ------------------

func main() {
	// Remove older versions immediately.
	removeOlderExecutables(currentVersion)

	// Set up logging.
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

	// Check for updates.
	checkAndUpdate()

	// Open keyboard once outside the loop.
	if err := keyboard.Open(); err != nil {
		log.Fatalf("Error opening keyboard: %v", err)
	}
	defer keyboard.Close()

	// SMB share loop.
	for {
		log.Println("Detecting drives...")
		fmt.Println("Detecting all local and USB drives...")

		drives, err := getDrivesSMB()
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
			fmt.Printf("[%d] %s (%s) - %s\n", i+1, drive.DeviceID, drive.VolumeName, driveTypeDescSMB(drive.DriveType))
		}

		var choice int
		fmt.Print("\nEnter the number of the drive you want to share: ")
		_, err = fmt.Scan(&choice)
		if err != nil || choice < 1 || choice > len(drives) {
			log.Fatalf("Invalid drive selection: %v", err)
		}
		log.Printf("Drive selected: %d", choice)

		selectedIP := getMainIPSMB()
		selectedDrive := drives[choice-1]
		// Force share name uppercase.
		shareName := strings.ToUpper(strings.Trim(selectedDrive.DeviceID, ":\\"))
		drivePath := strings.ToUpper(selectedDrive.DeviceID) + "\\"

		// Generate a random short user/pass.
		num, err := rand.Int(rand.Reader, big.NewInt(90))
		if err != nil {
			log.Fatalf("Failed to generate random number: %v", err)
		}
		randomNumber := num.Int64() + 10
		tempUser := fmt.Sprintf("smbuser%d", randomNumber)
		tempPass := fmt.Sprintf("SmbPass!%d", randomNumber)

		// Create the temp user.
		if err := createTempUser(tempUser, tempPass); err != nil {
			log.Fatalf("Error creating temp user: %v", err)
		}

		fmt.Printf("\nCreating SMB share '%s' for drive '%s'...\n", shareName, drivePath)
		log.Printf("Creating SMB share: Name=%s, DrivePath=%s, Using IP=%s, TempUser=%s", shareName, drivePath, selectedIP, tempUser)

		if err := createSMBShare(shareName, drivePath, tempUser); err != nil {
			_ = deleteTempUser(tempUser)
			log.Fatalf("Error creating SMB share: %v", err)
		}

		fmt.Printf("SMB Share '%s' created successfully!\n", shareName)
		fmt.Printf("Access it via \\localhost\\%s (if local) or \\\\%s\\%s (for remote users)\n", shareName, selectedIP, shareName)
		fmt.Printf("Use credentials:\n  Username: %s\n  Password: %s\n", tempUser, tempPass)

		fmt.Println("Press Ctrl+K to stop sharing...")
		log.Printf("SMB Share '%s' created. Awaiting Ctrl+K to remove share.", shareName)

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
		// Optionally, break out of outer loop to exit after one share session.
	}
}
