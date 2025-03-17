package main

import (
	"net"
	"os/exec"
	"strings"

	"github.com/StackExchange/wmi"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// Win32_LogicalDisk represents a drive detected by Windows WMI
// This structure is used to retrieve USB and local drives

type Win32_LogicalDisk struct {
	DeviceID   string
	VolumeName string
	DriveType  uint32
}

// getDrives queries WMI for all logical drives (USB and local)
func getDrives() ([]Win32_LogicalDisk, error) {
	var drives []Win32_LogicalDisk
	err := wmi.Query("SELECT DeviceID, VolumeName, DriveType FROM Win32_LogicalDisk WHERE DriveType=2 OR DriveType=3", &drives)
	return drives, err
}

// getLocalIPs retrieves all local IP addresses
func getLocalIPs() []string {
	var ipList []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{"localhost"}
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			ipList = append(ipList, ipNet.IP.String())
		}
	}

	if len(ipList) == 0 {
		ipList = append(ipList, "localhost")
	}

	return ipList
}

// createSMBShare creates an SMB share for a drive
func createSMBShare(name, path string) error {
	cmd := exec.Command("powershell", "New-SmbShare", "-Name", name, "-Path", path, "-FullAccess", "Everyone")
	return cmd.Run()
}

// removeSMBShare removes an existing SMB share
func removeSMBShare(name string) error {
	cmd := exec.Command("powershell", "Remove-SmbShare", "-Name", name, "-Force")
	return cmd.Run()
}

func main() {
	var mainWindow *walk.MainWindow
	var driveListBox *walk.ListBox
	var ipListBox *walk.ListBox

	// GUI Layout
	MainWindow{
		AssignTo: &mainWindow,
		Title:    "USB NAS Server GUI",
		Size:     Size{Width: 400, Height: 300},
		Layout:   VBox{},
		Children: []Widget{
			Label{Text: "Select Drive:"},
			ListBox{AssignTo: &driveListBox},
			Label{Text: "Select IP Address:"},
			ListBox{AssignTo: &ipListBox},
			PushButton{
				Text: "Share Drive",
				OnClicked: func() {
					selectedDriveIdx := driveListBox.CurrentIndex()
					selectedIPIdx := ipListBox.CurrentIndex()
					if selectedDriveIdx < 0 || selectedIPIdx < 0 {
						walk.MsgBox(mainWindow, "Error", "Please select both a drive and an IP.", walk.MsgBoxIconError)
						return
					}
					// Get selected values
					selectedDrive := driveListBox.Model().([]string)[selectedDriveIdx]
					selectedIP := ipListBox.Model().([]string)[selectedIPIdx]
					shareName := strings.Trim(selectedDrive, ":\\")
					drivePath := selectedDrive + "\\"
					// Create SMB Share
					err := createSMBShare(shareName, drivePath)
					if err != nil {
						walk.MsgBox(mainWindow, "Error", "Failed to create SMB share.", walk.MsgBoxIconError)
						return
					}
					walk.MsgBox(mainWindow, "Success", "Drive shared successfully!", walk.MsgBoxIconInformation)
				},
			},
		},
	}.Run()
}
