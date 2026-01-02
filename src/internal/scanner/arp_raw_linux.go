//go:build linux

package scanner

import (
	"encoding/binary"
	"net"
	"syscall"
	"unsafe"
)

// createRawSocketLinux creates an AF_PACKET raw socket for ARP
func createRawSocketLinux() (int, error) {
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_ARP)))
	if err != nil {
		return -1, err
	}
	return fd, nil
}

// closeRawSocket closes a raw socket
func closeRawSocket(fd int) {
	syscall.Close(fd)
}

// bindToInterface binds a raw socket to a specific interface
func bindToInterface(fd int, ifaceIndex int) error {
	addr := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ARP),
		Ifindex:  ifaceIndex,
	}
	return syscall.Bind(fd, &addr)
}

// sendARPRequest sends an ARP request packet
func sendARPRequest(fd int, srcMAC net.HardwareAddr, srcIP, dstIP net.IP) error {
	// Ethernet header (14 bytes)
	// ARP packet (28 bytes)
	packet := make([]byte, 42)

	// Ethernet header
	// Destination MAC: broadcast (ff:ff:ff:ff:ff:ff)
	for i := 0; i < 6; i++ {
		packet[i] = 0xff
	}
	// Source MAC
	copy(packet[6:12], srcMAC)
	// EtherType: ARP (0x0806)
	binary.BigEndian.PutUint16(packet[12:14], 0x0806)

	// ARP packet
	// Hardware type: Ethernet (1)
	binary.BigEndian.PutUint16(packet[14:16], 1)
	// Protocol type: IPv4 (0x0800)
	binary.BigEndian.PutUint16(packet[16:18], 0x0800)
	// Hardware size: 6
	packet[18] = 6
	// Protocol size: 4
	packet[19] = 4
	// Opcode: Request (1)
	binary.BigEndian.PutUint16(packet[20:22], 1)
	// Sender MAC
	copy(packet[22:28], srcMAC)
	// Sender IP
	copy(packet[28:32], srcIP.To4())
	// Target MAC: 00:00:00:00:00:00
	// packet[32:38] already zeroed
	// Target IP
	copy(packet[38:42], dstIP.To4())

	// Send packet
	addr := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ARP),
		Halen:    6,
	}
	copy(addr.Addr[:], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})

	return syscall.Sendto(fd, packet, 0, &addr)
}

// htons converts a uint16 from host to network byte order
func htons(i uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, i)
	return *(*uint16)(unsafe.Pointer(&b[0]))
}
