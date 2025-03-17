# USB NAS Server for Windows (Written in Go)

This project is an open-source, lightweight Windows application built with Go that automatically turns your USB drives into network-accessible storage (NAS) instantly upon connection.

![image](https://github.com/user-attachments/assets/6e59a6fa-c31b-47f7-b152-e65b2526d857)

## 🔥 Features

- Automatic USB detection and sharing via SMB.
- Easy-to-use, minimalist CLI interface.
- Secure by design with controlled access.
- Fully open-source and customizable.
- Logs activity for monitoring.

## 📦 Installation

### **1. Download the Binary**
You can download the latest release from the [Releases](https://github.com/MattInnovates/USB-NAS-Server-for-Windows/releases) page.

### **2. Manual Build (Optional)**
If you want to build from source, ensure you have Go installed and run:
```sh
go build -o usb-nas-cli.exe ./cmd
```

## 🚀 Usage

1. **Run the server** by executing:
   ```sh
   usb-nas-cli.exe
   ```
2. Plug in a USB drive and it will be automatically shared over the network.
3. Access the shared drive via SMB using:
   ```
   \\YOUR-PC-IP\YOUR-USB-DRIVE
   ```
4. Press `Ctrl + K` to stop sharing the drive.

## 🚧 Current Status

The first stable version has been released! Further improvements, Web UI, and security enhancements are planned.

## 🛠️ Tech Stack

- **Go (Golang)** - Core application
- **Windows SMB** - Network sharing
- **PowerShell Scripting** - Managing shares

## 🤝 Contributing

Feel free to fork, report issues, or propose improvements via Pull Requests.

## 📄 License

MIT License — Open Source, free to use.
