//go:build !linux && !darwin

package scanner

import (
	"fmt"
	"net"
)

// createRawSocketLinux is not available on this platform
func createRawSocketLinux() (int, error) {
	return -1, fmt.Errorf("raw sockets not implemented for this platform")
}

// closeRawSocket closes a raw socket
func closeRawSocket(fd int) {
	// No-op
}

// bindToInterface binds a raw socket to a specific interface
func bindToInterface(fd int, ifaceIndex int) error {
	return fmt.Errorf("raw sockets not implemented for this platform")
}

// sendARPRequest sends an ARP request packet
func sendARPRequest(fd int, srcMAC net.HardwareAddr, srcIP, dstIP net.IP) error {
	return fmt.Errorf("raw sockets not implemented for this platform")
}
