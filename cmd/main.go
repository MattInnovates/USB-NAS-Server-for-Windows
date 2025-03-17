// Package declaration for the main package.
package main

// Importing necessary libraries for the program.
import (
	"crypto/rand"   // Library for generating random numbers.
	"fmt"           // Library for formatted I/O operations.
	"io"            // Library for input/output operations.
	"io/ioutil"     // Library for input/output utility functions.
	"log"           // Library for logging.
	"math/big"      // Library for arbitrary-precision arithmetic.
	"net"           // Library for networking.
	"net/http"      // Library for HTTP protocol.
	"os"            // Library for operating system interactions.
	"os/exec"       // Library for executing external commands.
	"path/filepath" // Library for file path manipulation.
	"strings"       // Library for string manipulation.

	"github.com/Masterminds/semver/v3" // Library for semantic versioning.
	"github.com/StackExchange/wmi"     // Library for Windows Management Instrumentation.
	"github.com/eiannone/keyboard"     // Library for keyboard input.
)

// ------------------ CONFIG & VERSION ------------------

// Current version of the program (set via ldflags or manually).
var currentVersion = "1.0.0"

// URL hosting the text file with the latest version (e.g. "1.0.3").
var versionFileURL = "http://86.129.89.196/USB-NAS/latestVersion.txt"

// Base URL for the EXEs (we'll append "-vX.Y.Z.exe" at runtime).
var baseURL = "http://86.129.89.196/USB-NAS"

// ------------------ UPDATE LOGIC ------------------

// Function to check for updates and launch the new version if available.
func checkAndUpdate() {
	// Fetch the latest version from the version file URL.
	remoteVersionStr, err := fetchLatestVersion()
	if err != nil {
		// Log an error if unable to check for updates.
		log.Printf("Could not check for updates: %v", err)
		return
	}

	// Parse the local version string into a semantic version object.
	localSem, err := semver.NewVersion(currentVersion)
	if err != nil {
		// Log an error if the local version is invalid.
		log.Printf("Invalid local version: %v", err)
		return
	}
	// Parse the remote version string into a semantic version object.
	remoteSem, err := semver.NewVersion(remoteVersionStr)
	if err != nil {
		// Log an error if the remote version is invalid.
		log.Printf("Invalid remote version: %v", err)
		return
	}

	// Check if the remote version is greater than the local version.
	if remoteSem.GreaterThan(localSem) {
		// Log a message indicating that a new version is available.
		log.Printf("New version %s is available (current: %s).", remoteSem, localSem)
		// Prompt the user to update.
		fmt.Printf("A new version (%s) is available. Update now? (y/n): ", remoteSem)
		var input string
		// Read the user's input.
		fmt.Scanln(&input)
		if strings.ToLower(input) == "y" {
			// Download and launch the new version.
			if err := downloadAndLaunch(remoteSem.String()); err != nil {
				// Log a fatal error if the update fails.
				log.Fatalf("Update failed: %v", err)
			}
			// Exit the program after updating.
			os.Exit(0)
		}
	} else {
		// Log a message indicating that no updates are available.
		log.Printf("No updates found. Current version %s is up-to-date.", currentVersion)
	}
}

// Function to fetch the latest version from the version file URL.
func fetchLatestVersion() (string, error) {
	// Send an HTTP GET request to the version file URL.
	resp, err := http.Get(versionFileURL)
	if err != nil {
		// Return an error if the request fails.
		return "", err
	}
	// Close the response body when done.
	defer resp.Body.Close()

	// Check if the response status code is OK (200).
	if resp.StatusCode != http.StatusOK {
		// Return an error if the response status code is not OK.
		return "", fmt.Errorf("version check failed: HTTP %d", resp.StatusCode)
	}

	// Create a buffer to read the response body.
	buf := make([]byte, 32)
	// Read the response body into the buffer.
	n, err := resp.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		// Return an error if reading the response body fails.
		return "", err
	}
	// Trim the buffer to the actual length and return it as a string.
	return strings.TrimSpace(string(buf[:n])), nil
}

// Function to download and launch the new version.
func downloadAndLaunch(newVer string) error {
	// Construct the new executable name.
	newExeName := fmt.Sprintf("usb-nas-cli-v%s.exe", newVer)
	// Construct the download URL.
	downloadURL := fmt.Sprintf("%s/%s", baseURL, newExeName)
	// Log a message indicating that the new binary is being downloaded.
	log.Printf("Downloading new binary from: %s", downloadURL)

	// Send an HTTP GET request to the download URL.
	resp, err := http.Get(downloadURL)
	if err != nil {
		// Return an error if the request fails.
		return fmt.Errorf("failed to download new binary: %v", err)
	}
	// Close the response body when done.
	defer resp.Body.Close()

	// Check if the response status code is OK (200).
	if resp.StatusCode != http.StatusOK {
		// Return an error if the response status code is not OK.
		return fmt.Errorf("bad status downloading new binary: %d", resp.StatusCode)
	}

	// Get the path of the current executable.
	exePath, err := os.Executable()
	if err != nil {
		// Return an error if getting the executable path fails.
		return fmt.Errorf("could not get current exe path: %v", err)
	}
	// Get the directory of the current executable.
	exeDir := filepath.Dir(exePath)
	// Construct the full path of the new executable.
	newExeFullPath := filepath.Join(exeDir, newExeName)

	// Create a new file for the new executable.
	out, err := os.Create(newExeFullPath)
	if err != nil {
		// Return an error if creating the new file fails.
		return fmt.Errorf("failed to create new exe file: %v", err)
	}
	// Copy the response body to the new file.
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		// Close the new file and return an error if copying fails.
		out.Close()
		return fmt.Errorf("failed to write new exe: %v", err)
	}
	// Close the new file.
	out.Close()

	// Launch the new executable using PowerShell.
	cmd := exec.Command("powershell", "-Command", "Start-Process", newExeFullPath)
	if err := cmd.Start(); err != nil {
		// Return an error if launching the new executable fails.
		return fmt.Errorf("failed to launch new version: %v", err)
	}

	// Log a message indicating that the new version has been launched.
	log.Printf("Launched new version: %s", newExeFullPath)
	return nil
}

// Function to remove older executables.
func removeOlderExecutables(currentVerStr string) {
	// Parse the current version string into a semantic version object.
	currentSem, err := semver.NewVersion(currentVerStr)
	if err != nil {
		// Log an error if the current version is invalid.
		log.Printf("removeOlderExecutables: invalid current version: %v", err)
		return
	}

	// Read the files in the current directory.
	files, err := ioutil.ReadDir(".")
	if err != nil {
		// Log an error if reading the directory fails.
		log.Printf("removeOlderExecutables: failed to read dir: %v", err)
		return
	}

	// Iterate over the files in the directory.
	for _, f := range files {
		// Get the file name.
		name := f.Name()
		// Check if the file is an executable with a version suffix.
		if strings.HasPrefix(name, "usb-nas-cli-v") && strings.HasSuffix(name, ".exe") {
			// Extract the version suffix from the file name.
			verStr := strings.TrimSuffix(strings.TrimPrefix(name, "usb-nas-cli-v"), ".exe")
			// Parse the version suffix into a semantic version object.
			ver, err := semver.NewVersion(verStr)
			if err != nil {
				// Skip the file if the version suffix is invalid.
				continue
			}
			// Check if the version is less than the current version.
			if ver.LessThan(currentSem) {
				// Log a message indicating that the file is being removed.
				log.Printf("Removing older exe: %s", name)
				// Remove the file.
				os.Remove(name)
			}
		}
	}
}

// ------------------ SMB & TEMP USER LOGIC ------------------

// Structure to represent a logical disk.
type Win32_LogicalDisk struct {
	// Device ID of the disk (e.g., "C:").
	DeviceID string
	// Volume name of the disk (e.g., "System").
	VolumeName string
	// Drive type of the disk (2: Removable, 3: Local).
	DriveType uint32
}

// Function to get the logical disks.
func getDrivesSMB() ([]Win32_LogicalDisk, error) {
	// Create a slice to store the logical disks.
	var drives []Win32_LogicalDisk
	// Query WMI for the logical disks.
	err := wmi.Query("SELECT DeviceID, VolumeName, DriveType FROM Win32_LogicalDisk WHERE DriveType=2 OR DriveType=3", &drives)
	return drives, err
}

// Function to get a friendly drive type description.
func driveTypeDescSMB(driveType uint32) string {
	// Switch on the drive type.
	switch driveType {
	case 2:
		// Return "Removable (USB/SD)" for removable drives.
		return "Removable (USB/SD)"
	case 3:
		// Return "Local Disk" for local disks.
		return "Local Disk"
	default:
		// Return "Unknown" for unknown drive types.
		return "Unknown"
	}
}

// Function to get the main IP address.
func getMainIPSMB() string {
	// Dial 8.8.8.8 to get the main IP address.
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Log an error and return "localhost" if dialing fails.
		log.Printf("Error determining main IP: %v. Using localhost", err)
		return "localhost"
	}
	// Close the connection when done.
	defer conn.Close()
	// Get the local address of the connection.
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	// Get the IP address of the local address.
	ipStr := localAddr.IP.String()
	// Check if the IP address is 127.0.0.1.
	if ipStr == "127.0.0.1" {
		// Return "localhost" if the IP address is 127.0.0.1.
		return "localhost"
	}
	// Return the IP address.
	return ipStr
}

// Function to create a temporary local user.
func createTempUser(username, password string) error {
	// Log a message indicating that a temporary user is being created.
	log.Printf("Creating temp user: %s", username)
	// Create the temporary user using the "net user" command.
	cmd := exec.Command("net", "user", username, password, "/add")
	// Get the output of the command.
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Return an error if creating the user fails.
		return fmt.Errorf("failed to create user %s: %v\nOutput:\n%s", username, err, string(output))
	}
	// Add the user to the "Users" group using the "net localgroup" command.
	cmd = exec.Command("net", "localgroup", "Users", username, "/add")
	// Get the output of the command.
	groupOutput, groupErr := cmd.CombinedOutput()
	if groupErr != nil && !strings.Contains(string(groupOutput), "1378") {
		// Return an error if adding the user to the group fails.
		return fmt.Errorf("failed to add user %s to 'Users' group: %v\nOutput:\n%s", username, groupErr, string(groupOutput))
	}
	return nil
}

// Function to delete a temporary local user.
func deleteTempUser(username string) error {
	// Log a message indicating that a temporary user is being deleted.
	log.Printf("Deleting temp user: %s", username)
	// Delete the temporary user using the "net user" command.
	cmd := exec.Command("net", "user", username, "/delete")
	// Run the command and return any error.
	return cmd.Run()
}

// Function to create an SMB share.
func createSMBShare(shareName, drivePath, tempUser string) error {
	// Attempt to remove any existing share with the same name.
	_ = exec.Command("powershell", "-Command", "Remove-SmbShare", "-Name", shareName, "-Force").Run()

	// Build the FullAccess list: tempUser and Everyone.
	fullAccess := fmt.Sprintf("%s,Everyone", tempUser)
	// Log a message indicating that an SMB share is being created.
	log.Printf("Creating SMB share: %s -> %s, FullAccess=%s", shareName, drivePath, fullAccess)

	// Create the new share using the "New-SmbShare" command.
	cmd := exec.Command("powershell", "-Command", "New-SmbShare", "-Name", shareName, "-Path", drivePath, "-FullAccess", fullAccess)
	// Get the output of the command.
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Return an error if creating the share fails.
		return fmt.Errorf("failed to create SMB share: %v\nOutput: %s", err, string(out))
	}
	return nil
}

// Function to remove an SMB share.
func removeSMBShare(shareName string) error {
	// Log a message indicating that an SMB share is being removed.
	log.Printf("Removing SMB share: %s", shareName)
	// Remove the share using the "Remove-SmbShare" command.
	cmd := exec.Command("powershell", "-Command", "Remove-SmbShare", "-Name", shareName, "-Force")
	// Run the command and return any error.
	return cmd.Run()
}

// Function to check if the program is running as an administrator.
func isAdmin() bool {
	// Run the "net session" command to check if the program is running as an administrator.
	cmd := exec.Command("net", "session")
	// Return true if the command runs successfully, false otherwise.
	return cmd.Run() == nil
}

// Function to ensure the program is running as an administrator.
func ensureAdmin() {
	// Check if the program is running as an administrator.
	if !isAdmin() {
		// Log a message indicating that the program is not running as an administrator.
		log.Println("Not running as administrator. Restarting with elevated privileges...")
		// Get the path of the current executable.
		exePath, err := os.Executable()
		if err != nil {
			// Log a fatal error if getting the executable path fails.
			log.Fatalf("Failed to get exe path: %v", err)
		}
		// Launch the program with elevated privileges using PowerShell.
		cmd := exec.Command("powershell", "-Command", "Start-Process", exePath, "-Verb", "runas")
		if err := cmd.Start(); err != nil {
			// Log a fatal error if launching the program with elevated privileges fails.
			log.Fatalf("Failed to elevate privileges: %v", err)
		}
		// Exit the program.
		os.Exit(0)
	}
}

// ------------------ MAIN ------------------

// Main function of the program.
func main() {
	// Remove older versions immediately.
	removeOlderExecutables(currentVersion)

	// Set up logging.
	logFile, err := os.OpenFile("usb-nas-cli.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Print an error message if opening the log file fails.
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	// Close the log file when done.
	defer logFile.Close()
	// Set the log output to the log file.
	log.SetOutput(logFile)
	// Set the log flags to include the file name and line number.
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// Log a message indicating that the program has started.
	log.Printf("usb-nas-cli v%s started.", currentVersion)

	// Ensure the program is running as an administrator.
	ensureAdmin()

	// Check for updates.
	checkAndUpdate()

	// Open the keyboard once outside the loop.
	if err := keyboard.Open(); err != nil {
		// Log a fatal error if opening the keyboard fails.
		log.Fatalf("Error opening keyboard: %v", err)
	}
	// Close the keyboard when done.
	defer keyboard.Close()

	// SMB share loop.
	for {
		// Log a message indicating that drives are being detected.
		log.Println("Detecting drives...")
		// Print a message indicating that drives are being detected.
		fmt.Println("Detecting all local and USB drives...")

		// Get the logical disks.
		drives, err := getDrivesSMB()
		if err != nil {
			// Log a fatal error if getting the logical disks fails.
			log.Fatalf("Error detecting drives: %v", err)
		}

		// Check if any drives were detected.
		if len(drives) == 0 {
			// Print a message indicating that no drives were detected.
			fmt.Println("No drives detected.")
			// Log a message indicating that no drives were detected.
			log.Println("No drives detected. Exiting.")
			// Return from the main function.
			return
		}

		// Print a message indicating that drives were detected.
		fmt.Println("Drives Found:")
		// Iterate over the detected drives.
		for i, drive := range drives {
			// Print the drive information.
			fmt.Printf("[%d] %s (%s) - %s\n", i+1, drive.DeviceID, drive.VolumeName, driveTypeDescSMB(drive.DriveType))
		}

		// Declare a variable to store the user's choice.
		var choice int
		// Prompt the user to select a drive.
		fmt.Print("\nEnter the number of the drive you want to share: ")
		// Read the user's input.
		_, err = fmt.Scan(&choice)
		if err != nil || choice < 1 || choice > len(drives) {
			// Log a fatal error if the user's input is invalid.
			log.Fatalf("Invalid drive selection: %v", err)
		}
		// Log a message indicating that a drive has been selected.
		log.Printf("Drive selected: %d", choice)

		// Get the main IP address.
		selectedIP := getMainIPSMB()
		// Get the selected drive.
		selectedDrive := drives[choice-1]
		// Force the share name to uppercase.
		shareName := strings.ToUpper(strings.Trim(selectedDrive.DeviceID, ":\\"))
		// Get the drive path.
		drivePath := strings.ToUpper(selectedDrive.DeviceID) + "\\"

		// Generate a random short user and password.
		num, err := rand.Int(rand.Reader, big.NewInt(90))
		if err != nil {
			// Log a fatal error if generating a random number fails.
			log.Fatalf("Failed to generate random number: %v", err)
		}
		// Calculate the random number.
		randomNumber := num.Int64() + 10
		// Construct the temporary user name.
		tempUser := fmt.Sprintf("smbuser%d", randomNumber)
		// Construct the temporary password.
		tempPass := fmt.Sprintf("SmbPass!%d", randomNumber)

		// Create the temporary user.
		if err := createTempUser(tempUser, tempPass); err != nil {
			// Log a fatal error if creating the temporary user fails.
			log.Fatalf("Error creating temp user: %v", err)
		}

		// Print a message indicating that an SMB share is being created.
		fmt.Printf("\nCreating SMB share '%s' for drive '%s'...\n", shareName, drivePath)
		// Log a message indicating that an SMB share is being created.
		log.Printf("Creating SMB share: Name=%s, DrivePath=%s, Using IP=%s, TempUser=%s", shareName, drivePath, selectedIP, tempUser)

		// Create the SMB share.
		if err := createSMBShare(shareName, drivePath, tempUser); err != nil {
			// Delete the temporary user if creating the SMB share fails.
			_ = deleteTempUser(tempUser)
			// Log a fatal error if creating the SMB share fails.
			log.Fatalf("Error creating SMB share: %v", err)
		}

		// Print a message indicating that the SMB share has been created.
		fmt.Printf("SMB Share '%s' created successfully!\n", shareName)
		// Print a message indicating how to access the SMB share.
		fmt.Printf("Access it via \\localhost\\%s (if local) or \\\\%s\\%s (for remote users)\n", shareName, selectedIP, shareName)
		// Print a message indicating the credentials to use.
		fmt.Printf("Use credentials:\n  Username: %s\n  Password: %s\n", tempUser, tempPass)

		// Print a message indicating how to stop the SMB share.
		fmt.Println("Press Ctrl+K to stop sharing...")
		// Log a message indicating that the SMB share is awaiting Ctrl+K to remove.
		log.Printf("SMB Share '%s' created. Awaiting Ctrl+K to remove share.", shareName)

		// Loop until Ctrl+K is pressed.
		for {
			// Get the key pressed.
			_, key, err := keyboard.GetKey()
			if err != nil {
				// Log a fatal error if getting the key pressed fails.
				log.Fatalf("Error reading keyboard input: %v", err)
			}
			// Check if Ctrl+K was pressed.
			if key == keyboard.KeyCtrlK {
				// Print a message indicating that the SMB share is being stopped.
				fmt.Println("\nStopping SMB share...")
				// Log a message indicating that Ctrl+K was detected.
				log.Printf("Ctrl+K detected. Stopping SMB share '%s'", shareName)

				// Remove the SMB share.
				if err := removeSMBShare(shareName); err != nil {
					// Log a fatal error if removing the SMB share fails.
					log.Fatalf("Error removing SMB share: %v", err)
				}

				// Delete the temporary user.
				if err := deleteTempUser(tempUser); err != nil {
					// Log an error if deleting the temporary user fails.
					log.Printf("Error deleting temp user: %v", err)
				}

				// Print a message indicating that the SMB share has been stopped.
				fmt.Println("SMB share stopped successfully.\n")
				// Log a message indicating that the SMB share has been removed.
				log.Printf("SMB share '%s' removed successfully.", shareName)
				// Break out of the loop.
				break
			}
		}
		// Optionally, break out of the outer loop to exit after one share session.
	}
}
