package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/StackExchange/wmi"
	"github.com/eiannone/keyboard"
)

// Win32_LogicalDisk represents the structure for querying disk information from WMI.
type Win32_LogicalDisk struct {
	DeviceID   string // Drive letter (e.g., C:)
	VolumeName string // Drive label
	DriveType  uint32 // 2: Removable, 3: Local disk
}

// getDrives queries WMI for all logical drives of type removable (USB) or local disk.
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

// getMainIP returns the primary IP by dialing an external address (e.g., 8.8.8.8:80).
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

// createSMBShare creates an SMB share for the specified drive using PowerShell.
func createSMBShare(name, path string) error {
	log.Printf("Attempting to create SMB share: Name=%s, Path=%s", name, path)
	// Grant full access to both Everyone and Guest.
	cmd := exec.Command("powershell", "New-SmbShare", "-Name", name, "-Path", path, "-FullAccess", "Everyone,Guest")
	err := cmd.Run()
	if err != nil {
		log.Printf("Error creating SMB share: %v", err)
	}
	return err
}

// removeSMBShare removes an existing SMB share by name using PowerShell.
func removeSMBShare(name string) error {
	log.Printf("Attempting to remove SMB share: Name=%s", name)
	cmd := exec.Command("powershell", "Remove-SmbShare", "-Name", name, "-Force")
	err := cmd.Run()
	if err != nil {
		log.Printf("Error removing SMB share: %v", err)
	}
	return err
}

// isAdmin checks if the program is running with administrative privileges by using the "net session" trick.
func isAdmin() bool {
	cmd := exec.Command("net", "session")
	err := cmd.Run()
	return err == nil
}

// ensureAdmin attempts to restart the program with elevated privileges if not already running as admin.
func ensureAdmin() {
	if !isAdmin() {
		log.Println("Not running as administrator. Attempting to restart with elevated privileges...")
		exePath, err := os.Executable()
		if err != nil {
			log.Fatalf("Failed to get executable path: %v", err)
		}
		// Relaunch using PowerShell's Start-Process with the -Verb runas (elevated)
		cmd := exec.Command("powershell", "-Command", "Start-Process", exePath, "-Verb", "runas")
		err = cmd.Start()
		if err != nil {
			log.Fatalf("Failed to elevate privileges: %v", err)
		}
		os.Exit(0)
	}
}

func main() {
	// Set up logging to file usb-nas-cli.log
	logFile, err := os.OpenFile("usb-nas-cli.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("usb-nas-cli Debug Build started.")

	// Ensure the program is running as administrator.
	ensureAdmin()

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

		// Automatically determine the main IP from the active network card.
		selectedIP := getMainIP()

		selectedDrive := drives[choice-1]
		shareName := strings.Trim(selectedDrive.DeviceID, ":\\")
		drivePath := selectedDrive.DeviceID + "\\"

		fmt.Printf("\nCreating SMB share '%s' for drive '%s'...\n", shareName, drivePath)
		log.Printf("Creating SMB share: Name=%s, DrivePath=%s, Using IP=%s", shareName, drivePath, selectedIP)
		err = createSMBShare(shareName, drivePath)
		if err != nil {
			log.Fatalf("Error creating SMB share: %v", err)
		}

		fmt.Printf("SMB Share '%s' created successfully! Access it via \\\\%s\\%s\n", shareName, selectedIP, shareName)
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
				err := removeSMBShare(shareName)
				if err != nil {
					log.Fatalf("Error removing SMB share: %v", err)
				}
				fmt.Println("SMB share stopped successfully.\n")
				log.Printf("SMB share '%s' removed successfully.", shareName)
				break
			}
		}
	}
}
