package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net"
	"os/exec"
	"strings"

	"github.com/StackExchange/wmi"
)

type Win32_LogicalDisk struct {
	DeviceID   string
	VolumeName string
	DriveType  uint32
}

type SMBLogic struct{}

var SMBLogicInstance = &SMBLogic{}

// getDrivesSMB uses WMI to fetch logical disk information.
func (s *SMBLogic) getDrivesSMB() ([]Win32_LogicalDisk, error) {
	var drives []Win32_LogicalDisk
	err := wmi.Query("SELECT DeviceID, VolumeName, DriveType FROM Win32_LogicalDisk WHERE DriveType=2 OR DriveType=3", &drives)
	return drives, err
}

// driveTypeDescSMB returns a human-readable drive type description.
func (s *SMBLogic) driveTypeDescSMB(driveType uint32) string {
	switch driveType {
	case 2:
		return "Removable (USB/SD)"
	case 3:
		return "Local Disk"
	default:
		return "Unknown"
	}
}

// getMainIPSMB determines the main IP address by opening a UDP connection.
func (s *SMBLogic) getMainIPSMB() string {
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

// createTempUser creates a temporary user using Windows net commands.
func (s *SMBLogic) createTempUser(username, password string) error {
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

// deleteTempUser deletes the temporary user.
func (s *SMBLogic) deleteTempUser(username string) error {
	log.Printf("Deleting temp user: %s", username)
	cmd := exec.Command("net", "user", username, "/delete")
	return cmd.Run()
}

// createSMBShare creates an SMB share using PowerShell.
func (s *SMBLogic) createSMBShare(shareName, drivePath, tempUser string) error {
	// Remove any existing share with the same name.
	_ = exec.Command("powershell", "-Command", "Remove-SmbShare", "-Name", shareName, "-Force").Run()
	fullAccess := fmt.Sprintf("%s,Everyone", tempUser)
	log.Printf("Creating SMB share: %s -> %s, FullAccess=%s", shareName, drivePath, fullAccess)
	cmd := exec.Command("powershell", "-Command", "New-SmbShare", "-Name", shareName, "-Path", drivePath, "-FullAccess", fullAccess)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create SMB share: %v\nOutput: %s", err, string(out))
	}
	return nil
}

// removeSMBShare removes the SMB share.
func (s *SMBLogic) removeSMBShare(shareName string) error {
	log.Printf("Removing SMB share: %s", shareName)
	cmd := exec.Command("powershell", "-Command", "Remove-SmbShare", "-Name", shareName, "-Force")
	return cmd.Run()
}

// generateTempCredentials creates temporary user credentials with a simple random number.
func generateTempCredentials() (string, string, error) {
	num, err := rand.Int(rand.Reader, big.NewInt(90))
	if err != nil {
		return "", "", err
	}
	randomNumber := num.Int64() + 10
	tempUser := fmt.Sprintf("smbuser%d", randomNumber)
	tempPass := fmt.Sprintf("SmbPass!%d", randomNumber)
	return tempUser, tempPass, nil
}
