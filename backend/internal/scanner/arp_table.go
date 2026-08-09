package scanner

import (
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// arpCacheTTL bounds how stale a cached ARP snapshot may be. ARP entries appear
// as a side effect of probing, so the snapshot has to refresh during a sweep to
// pick up hosts resolved moments ago — but not so often that we are back to
// spawning a process per address.
const arpCacheTTL = 2 * time.Second

var (
	arpCacheMu sync.Mutex
	arpCache   map[string]string
	arpCacheAt time.Time
)

// arpTable returns a recent snapshot of the host's ARP table, keyed by IPv4
// address with upper-case MAC values. Incomplete entries are omitted.
func arpTable() map[string]string {
	arpCacheMu.Lock()
	defer arpCacheMu.Unlock()

	if arpCache != nil && time.Since(arpCacheAt) < arpCacheTTL {
		return arpCache
	}

	arpCache = readARPTable()
	arpCacheAt = time.Now()
	return arpCache
}

// invalidateARPCache forces the next arpTable call to re-read the table. Used
// at the start of a sweep so a scan never opens with a stale snapshot.
func invalidateARPCache() {
	arpCacheMu.Lock()
	defer arpCacheMu.Unlock()
	arpCache = nil
}

func readARPTable() map[string]string {
	switch runtime.GOOS {
	case "linux":
		return readARPTableLinux()
	case "darwin":
		return readARPTableBSD()
	case "windows":
		return readARPTableWindows()
	default:
		return map[string]string{}
	}
}

func readARPTableLinux() map[string]string {
	table := map[string]string{}

	content, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return table
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines[1:] { // skip header
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if mac := normalizeMAC(fields[3]); mac != "" {
			table[fields[0]] = mac
		}
	}
	return table
}

// bsdARPLine matches "? (192.168.1.1) at aa:bb:cc:dd:ee:ff on en0 ..." as
// emitted by arp -an on macOS and the BSDs. Incomplete entries render the MAC
// as "(incomplete)" and fail the second group, so they are skipped.
var bsdARPLine = regexp.MustCompile(`\(([0-9.]+)\) at ([0-9a-fA-F:]+)`)

func readARPTableBSD() map[string]string {
	table := map[string]string{}

	output, err := exec.Command("arp", "-an").Output()
	if err != nil {
		return table
	}

	for _, match := range bsdARPLine.FindAllStringSubmatch(string(output), -1) {
		if mac := normalizeMAC(match[2]); mac != "" {
			table[match[1]] = mac
		}
	}
	return table
}

func readARPTableWindows() map[string]string {
	table := map[string]string{}

	output, err := exec.Command("arp", "-a").Output()
	if err != nil {
		return table
	}

	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if mac := normalizeMAC(strings.ReplaceAll(fields[1], "-", ":")); mac != "" {
			table[fields[0]] = mac
		}
	}
	return table
}

// normalizeMAC returns the upper-case form of a well-formed MAC, or "" for
// anything malformed, incomplete or all-zero.
func normalizeMAC(mac string) string {
	mac = strings.TrimSpace(mac)
	if len(mac) != 17 || strings.Count(mac, ":") != 5 {
		return ""
	}
	mac = strings.ToUpper(mac)
	if mac == "00:00:00:00:00:00" || mac == "FF:FF:FF:FF:FF:FF" {
		return ""
	}
	return mac
}
