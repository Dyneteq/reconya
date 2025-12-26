# Experimental Docker Support

⚠️ **Warning: These Docker files are experimental and have known limitations.**

## Known Issues

The fundamental limitation is Docker's network architecture. Even with comprehensive MAC discovery methods, privileged mode, and enhanced capabilities, Docker containers cannot reliably access Layer 2 (MAC address) information across different network segments.

### Specific Limitations:
- **MAC Address Discovery**: Containers cannot see MAC addresses of devices on different network segments
- **Network Isolation**: Docker's virtualized networking prevents full Layer 2 visibility
- **Reduced Functionality**: Device discovery works but lacks complete vendor identification

## Files in this Directory

- `Dockerfile` - Container definition
- `docker-compose.yml` - Standard Docker composition
- `docker-compose.host.yml` - Host networking override (still limited)
- `setup.sh` - Automated Docker setup script
- `start.sh` - Container startup script
- `stop.sh` - Container shutdown script  
- `logs.sh` - Container log viewing script

## If You Want to Experiment

**Note: This is not recommended for production use.**

1. **Quick Start:**
   ```bash
   ./setup.sh
   ./start.sh
   ```

2. **Access:** `http://localhost:3001`
3. **Login:** `admin` / `password`

4. **Stop:** `./stop.sh`
5. **View logs:** `./logs.sh`

## Recommended Alternative

For full functionality including complete MAC address discovery, use the local installation method described in the main README.md file.

## Why These Limitations Exist

Docker containers run in isolated network namespaces. Even with:
- `network_mode: host`
- `privileged: true` 
- `NET_ADMIN` and `NET_RAW` capabilities
- Advanced ARP scanning tools

The container still cannot access Layer 2 (MAC address) information across different network segments due to the fundamental architecture of Docker networking.

## Future Improvements

These files are kept for future experimentation and potential solutions such as:
- Custom network drivers
- Host-based helper services
- Alternative container runtimes
- Hybrid deployment models

For now, local installation provides the best user experience.