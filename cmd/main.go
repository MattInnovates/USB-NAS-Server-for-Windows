package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/eiannone/keyboard"
)

func main() {
	// Print current version and log the startup.
	fmt.Printf("Current Version: v%s\n", currentVersion)
	log.Printf("usb-nas-cli v%s started.", currentVersion)

	// Remove older versions.
	UpdateLogicInstance.removeOlderExecutables(currentVersion)

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

	// Ensure the program is running as an administrator.
	ensureAdmin()

	// Check for updates.
	UpdateLogicInstance.CheckAndUpdate()

	// Open the keyboard for input.
	if err := keyboard.Open(); err != nil {
		log.Fatalf("Error opening keyboard: %v", err)
	}
	defer keyboard.Close()

	// Begin the SMB share loop.
	for {
		log.Println("Detecting drives...")
		fmt.Println("Detecting all local and USB drives...")

		drives, err := SMBLogicInstance.getDrivesSMB()
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
			fmt.Printf("[%d] %s (%s) - %s\n", i+1, drive.DeviceID, drive.VolumeName, SMBLogicInstance.driveTypeDescSMB(drive.DriveType))
		}

		// Integer-based drive selection.
		var choice int
		fmt.Print("\nEnter the number of the drive you want to share: ")
		n, err := fmt.Scan(&choice)
		if err != nil {
			// If the user hits Ctrl+D or enters invalid input, exit.
			if err.Error() == "EOF" || n == 0 {
				log.Println("No valid input provided, exiting.")
				return
			}
			log.Fatalf("Invalid drive selection: %v", err)
		}
		if choice < 1 || choice > len(drives) {
			log.Fatalf("Invalid drive selection: %v", choice)
		}
		log.Printf("Drive selected: %d", choice)

		selectedIP := SMBLogicInstance.getMainIPSMB()
		selectedDrive := drives[choice-1]
		shareName := strings.ToUpper(strings.Trim(selectedDrive.DeviceID, ":\\"))
		drivePath := strings.ToUpper(selectedDrive.DeviceID) + "\\"

		// Generate temporary user credentials.
		tempUser, tempPass, err := generateTempCredentials()
		if err != nil {
			log.Fatalf("Error generating temporary credentials: %v", err)
		}

		if err := SMBLogicInstance.createTempUser(tempUser, tempPass); err != nil {
			log.Fatalf("Error creating temp user: %v", err)
		}

		fmt.Printf("\nCreating SMB share '%s' for drive '%s'...\n", shareName, drivePath)
		log.Printf("Creating SMB share: Name=%s, DrivePath=%s, Using IP=%s, TempUser=%s", shareName, drivePath, selectedIP, tempUser)

		if err := SMBLogicInstance.createSMBShare(shareName, drivePath, tempUser); err != nil {
			_ = SMBLogicInstance.deleteTempUser(tempUser)
			log.Fatalf("Error creating SMB share: %v", err)
		}

		fmt.Printf("SMB Share '%s' created successfully!\n", shareName)
		fmt.Printf("Access it via \\\\localhost\\%s (if local) or \\\\%s\\%s (for remote users)\n", shareName, selectedIP, shareName)
		fmt.Printf("Use credentials:\n  Username: %s\n  Password: %s\n", tempUser, tempPass)
		fmt.Println("Press Ctrl+K to stop sharing...")
		log.Printf("SMB Share '%s' created. Awaiting Ctrl+K to remove share.", shareName)

		// Wait for Ctrl+K to stop the share.
		for {
			_, key, err := keyboard.GetKey()
			if err != nil {
				log.Fatalf("Error reading keyboard input: %v", err)
			}
			if key == keyboard.KeyCtrlK {
				fmt.Println("\nStopping SMB share...")
				log.Printf("Ctrl+K detected. Stopping SMB share '%s'", shareName)
				if err := SMBLogicInstance.removeSMBShare(shareName); err != nil {
					log.Fatalf("Error removing SMB share: %v", err)
				}
				if err := SMBLogicInstance.deleteTempUser(tempUser); err != nil {
					log.Printf("Error deleting temp user: %v", err)
				}
				fmt.Println("SMB share stopped successfully.")
				log.Printf("SMB share '%s' removed successfully.", shareName)
				break
			}
		}
	}
}
