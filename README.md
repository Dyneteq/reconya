# Reconya

Network reconnaissance and asset discovery tool built with Go and HTMX.

![Dashboard Screenshot](screenshots/dashboard.png)

## Overview

Reconya discovers and monitors devices on your network with real-time updates. Suitable for network administrators, security professionals, and home users.

### Features

- **IPv4 Network Scanning** - Comprehensive device discovery with nmap integration
- **IPv6 Passive Monitoring** - Detects IPv6 devices through neighbor discovery and interface monitoring
- **Device Identification** - MAC addresses, vendor detection, hostnames, and device types
- **Dual-Stack Support** - Full IPv4 and IPv6 address display and management
- **Real-time Monitoring** - Live device status updates and event logging
- **Web-based Dashboard** - Modern HTMX-powered interface with dark theme
- **Device Fingerprinting** - Automatic OS and device type detection
- **Network Management** - Multi-network support with CIDR configuration

## Community

Join our community for support, discussions, and updates:

[![Discord](https://img.shields.io/badge/Discord-Join%20Community-7289da?style=for-the-badge&logo=discord&logoColor=white)](https://discord.gg/JW7VtBnNXp)  
[![Reddit](https://img.shields.io/badge/Reddit-r/reconya-ff4500?style=for-the-badge&logo=reddit&logoColor=white)](https://www.reddit.com/r/reconya/)

## Important Notice: Docker Implementation Status

⚠️ **Docker networking has been moved to experimental status due to fundamental limitations.**

The fundamental limitation is Docker's network architecture. Even with comprehensive MAC discovery methods, privileged mode, and enhanced capabilities, Docker containers cannot reliably access Layer 2 (MAC address) information across different network segments.

**For full functionality, including complete MAC address discovery, please use the local installation method below.**

Docker files have been moved to the `experimental/` directory for those who want to experiment with containerized deployment, but local installation is the recommended approach.

## Prerequisites

Before installing reconYa, ensure you have the following installed on your system:

- **Go 1.21 or later** - [Download Go](https://golang.org/dl/)
- **nmap** - Network scanning tool (instructions below)
- **make** - Build tool (pre-installed on most Unix systems)

## Local Installation (Recommended)

### One-Command Installation

```bash
git clone https://github.com/Dyneteq/reconya.git
cd reconya
make install
```

This will:
- Download Go dependencies
- Create default `.env` configuration file

**After installation, use these commands:**
```bash
make start       # Start reconYa as daemon
make start-dev   # Start in foreground (dev mode)
make stop        # Stop reconYa
make status      # Check service status
make logs        # View logs
make help        # Show all commands
```

Then open your browser to: `http://localhost:3008`
Default login: `admin` / `password`

### Manual Installation

If you prefer to install manually:

#### Prerequisites

1. **Install Go** (1.21 or later): https://golang.org/dl/
2. **Install nmap**:
   ```bash
   # macOS
   brew install nmap

   # Ubuntu/Debian
   sudo apt-get install nmap

   # RHEL/CentOS/Fedora
   sudo yum install nmap  # or dnf install nmap
   ```

3. **Grant nmap privileges** (for MAC address detection):
   ```bash
   sudo chown root:admin $(which nmap)
   sudo chmod u+s $(which nmap)
   ```

#### Setup & Run

1. **Clone the repository:**
   ```bash
   git clone https://github.com/Dyneteq/reconya.git
   cd reconya
   ```

2. **Setup backend:**
   ```bash
   cd backend
   cp .env.example .env
   # Edit .env file to set your credentials
   go mod download
   ```

3. **Start the application:**
   ```bash
   cd backend
   go run ./cmd
   ```

   **Windows users:** If you encounter SQLite CGO errors, use:
   ```bash
   cd backend
   CGO_ENABLED=1 go run ./cmd
   ```

4. **Access the application:**
   - Open your browser to: `http://localhost:3008`
   - Default login: `admin` / `password` (check your `.env` file for custom credentials)

## How to Use

1. Login with your credentials (default: `admin` / `password`)
2. Set up a new network
3. Choose the network from the dropdown and start scan
4. Devices will automatically appear as they're discovered on your network
5. Click on devices to see details including:
   - MAC addresses and vendor information
   - Open ports and running services
   - Operating system fingerprints
   - Device screenshots (for web services)
6. Use the network map to visualize device locations
7. Monitor the event log for network activity

## IPv6 Passive Monitoring

reconYa includes advanced IPv6 passive monitoring capabilities that activate automatically during network scans:

### How It Works
- **Neighbor Discovery Protocol (NDP)** - Monitors IPv6 neighbor cache for active devices
- **Interface Monitoring** - Detects IPv6 addresses on network interfaces
- **Automatic Classification** - Identifies Link-Local, Unique Local, and Global addresses
- **Dual-Stack Integration** - Links IPv6 addresses to existing IPv4 devices via MAC addresses

### IPv6 Address Types
- **Link-Local** (`fe80::/10`) - Local network segment addresses
- **Unique Local** (`fc00::/7`) - Private network addresses  
- **Global** (`2000::/3`) - Internet-routable addresses

### Features
- **Passive Detection** - No network traffic generated, only monitors existing traffic
- **Real-time Updates** - IPv6 addresses appear in device list and details
- **Cross-Platform** - Works on Linux, macOS, and Windows
- **Automatic Activation** - Starts with scanning, stops when idle

## Configuration

Edit the `backend/.env` file to customize:

```bash
LOGIN_USERNAME=admin
LOGIN_PASSWORD=your_secure_password
DATABASE_NAME="reconya-dev"
JWT_SECRET_KEY="your_jwt_secret"
SQLITE_PATH="data/reconya-dev.db"

# IPv6 Monitoring Configuration
IPV6_MONITORING_ENABLED=true
IPV6_MONITOR_INTERFACES=
IPV6_MONITOR_INTERVAL=30
IPV6_LINK_LOCAL_MONITORING=true
IPV6_MULTICAST_MONITORING=false
```

## Architecture

- **Backend**: Go API with HTMX templates and SQLite database (Port 3008)
- **Web Interface**: HTML and vanilla JS
- **Scanning**: Multi-strategy network discovery with nmap integration
- **Database**: SQLite for device storage and event logging

## Scanning Algorithm

### Discovery Process

Reconya uses a multi-layered scanning approach that combines nmap integration with native Go implementations:

**1. Network Discovery (Every 30 seconds)**
- Multiple nmap strategies with automatic fallback
- ICMP ping sweeps (privileged mode)
- TCP connect probes to common ports (fallback)
- ARP table lookups for MAC address resolution

**2. Device Identification**
- IEEE OUI database for vendor identification
- Multi-method hostname resolution (DNS, NetBIOS, mDNS)
- Operating system fingerprinting via nmap
- Device type classification based on ports and vendors

**3. Port Scanning (Background workers)**
- Top 100 ports scan for active services
- Service detection and banner grabbing
- Concurrent scanning with worker pool pattern

**4. Web Service Detection**
- Automatic discovery of HTTP/HTTPS services
- Screenshot capture using headless Chrome
- Service metadata extraction (titles, server headers)

## Troubleshooting

### Common Issues

**No devices found**
- Run `make status` to check if nmap is installed and configured
- Check that you're on the same network segment as target devices

**Services won't start**
- Run `make stop` to kill any stuck processes
- Check `make status` for dependency issues
- Ensure port 3008 is available

**Missing MAC addresses**
- Run `make status` to verify nmap permissions
- MAC addresses only visible on same network segment
- Some devices may not respond to ARP requests

**Permission denied errors**
- Run `make setup-nmap` to configure nmap permissions
- Or manually run: `sudo chmod u+s $(which nmap)`

**Services keep crashing**
- Verify your `.env` configuration is correct
- Try stopping and restarting: `make stop && make start`
- Check logs with: `make logs`

**Windows SQLite CGO Error**
- If you see "Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work":
  ```bash
  cd backend
  CGO_ENABLED=1 go run ./cmd
  # or for building:
  make build-cgo
  ```
- Ensure you have a C compiler installed (like TDM-GCC or Visual Studio Build Tools)

## Uninstalling reconYa

To completely remove reconYa:

```bash
make stop      # Stop any running processes
make clean     # Remove build artifacts
rm -rf reconya # Remove the directory
```

## Experimental Docker Support

Docker files are available in the `experimental/` directory but are not recommended due to network isolation limitations that prevent proper MAC address discovery. Use local installation for full functionality.

## Contributing

1. Fork the repository
2. Create feature branch
3. Make changes and test
4. Submit pull request

## License

Creative Commons Attribution-NonCommercial 4.0 International License. Commercial use requires permission.

## 🌟 Please check my other projects!

- **[tududi](https://tududi.com)** -  Self-hosted task management with hierarchical organization, multi-language support, and Telegram integration
- **[BreachHarbor](https://breachharbor.com)** - Cybersecurity suite for digital asset protection  
- **[Hevetra](https://hevetra.com)** - Digital tracking for child health milestones