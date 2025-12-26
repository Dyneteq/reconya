package models

import "encoding/xml"

// NmapXML represents the top-level structure of the Nmap XML output
type NmapXML struct {
	XMLName xml.Name      `xml:"nmaprun"`
	Hosts   []NmapXMLHost `xml:"host"`
}

// NmapXMLAddress represents an address (IP or MAC) in the Nmap XML output
type NmapXMLAddress struct {
	AddrType string `xml:"addrtype,attr"` // e.g., "ipv4" or "mac"
	Addr     string `xml:"addr,attr"`     // The actual address value
	Vendor   string `xml:"vendor,attr"`   // The vendor of the device (optional)
}

// NmapXMLHostname represents a hostname in the Nmap XML output
type NmapXMLHostname struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

// Update NmapXMLHost to include Addresses
type NmapXMLHost struct {
	Addresses []NmapXMLAddress  `xml:"address"` // Add this line to include address information
	Ports     []NmapXMLPort     `xml:"ports>port"`
	Hostnames []NmapXMLHostname `xml:"hostnames>hostname"`
}

// NmapXMLPort represents a port in the Nmap XML output
type NmapXMLPort struct {
	Protocol string         `xml:"protocol,attr"`
	PortID   string         `xml:"portid,attr"`
	State    NmapXMLState   `xml:"state"`
	Service  NmapXMLService `xml:"service"`
}

// NmapXMLState represents the state of a port in the Nmap XML output
type NmapXMLState struct {
	State string `xml:"state,attr"`
}

// NmapXMLService represents the service of a port in the Nmap XML output
type NmapXMLService struct {
	Name string `xml:"name,attr"`
}
