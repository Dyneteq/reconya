package oui

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// OUIService handles MAC address to vendor lookup using IEEE OUI database
type OUIService struct {
	ouiMap   map[string]string
	mutex    sync.RWMutex
	dataPath string
}

// NewOUIService creates a new OUI service instance
func NewOUIService(dataPath string) *OUIService {
	return &OUIService{
		ouiMap:   make(map[string]string),
		dataPath: dataPath,
	}
}

// Initialize downloads OUI database if needed and loads it into memory
func (s *OUIService) Initialize() error {
	// Ensure data directory exists
	if err := os.MkdirAll(s.dataPath, 0755); err != nil {
		return fmt.Errorf("failed to create OUI data directory: %w", err)
	}

	ouiFile := filepath.Join(s.dataPath, "oui.txt")
	
	// Check if we need to download/update the OUI database
	if s.shouldUpdateOUI(ouiFile) {
		log.Println("Downloading IEEE OUI database...")
		if err := s.downloadOUIDatabase(ouiFile); err != nil {
			log.Printf("Failed to download OUI database: %v", err)
			// Continue with existing file if download fails
		} else {
			log.Println("IEEE OUI database downloaded successfully")
		}
	}
	
	// Load OUI database into memory
	if err := s.loadOUIDatabase(ouiFile); err != nil {
		return fmt.Errorf("failed to load OUI database: %w", err)
	}
	
	log.Printf("Loaded %d OUI entries into memory", len(s.ouiMap))
	return nil
}

// shouldUpdateOUI checks if we need to download/update the OUI database
func (s *OUIService) shouldUpdateOUI(ouiFile string) bool {
	info, err := os.Stat(ouiFile)
	if err != nil {
		// File doesn't exist, need to download
		return true
	}
	
	// Update if file is older than 30 days
	return time.Since(info.ModTime()) > 30*24*time.Hour
}

// downloadOUIDatabase downloads the IEEE OUI database
func (s *OUIService) downloadOUIDatabase(ouiFile string) error {
	// IEEE OUI database URL
	url := "http://standards-oui.ieee.org/oui/oui.txt"
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download OUI database: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download OUI database: HTTP %d", resp.StatusCode)
	}
	
	// Create temporary file
	tempFile := ouiFile + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()
	
	// Copy response body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to write OUI database: %w", err)
	}
	
	// Atomically replace the old file with the new one
	if err := os.Rename(tempFile, ouiFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to replace OUI database: %w", err)
	}
	
	return nil
}

// loadOUIDatabase loads the OUI database into memory
func (s *OUIService) loadOUIDatabase(ouiFile string) error {
	file, err := os.Open(ouiFile)
	if err != nil {
		return fmt.Errorf("failed to open OUI database: %w", err)
	}
	defer file.Close()
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// Clear existing map
	s.ouiMap = make(map[string]string)
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Look for lines with OUI assignments
		// Format: "00-00-00   (hex)		XEROX CORPORATION"
		if strings.Contains(line, "(hex)") {
			parts := strings.SplitN(line, "(hex)", 2)
			if len(parts) == 2 {
				oui := strings.TrimSpace(parts[0])
				vendor := strings.TrimSpace(parts[1])
				
				// Normalize OUI format (remove dashes, convert to uppercase)
				oui = strings.ReplaceAll(oui, "-", "")
				oui = strings.ToUpper(oui)
				
				if len(oui) == 6 && vendor != "" {
					s.ouiMap[oui] = vendor
				}
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read OUI database: %w", err)
	}
	
	return nil
}

// LookupVendor returns the vendor name for a given MAC address
func (s *OUIService) LookupVendor(macAddress string) string {
	if macAddress == "" {
		return ""
	}
	
	// Extract OUI (first 6 characters after removing separators)
	oui := s.extractOUI(macAddress)
	if oui == "" {
		return ""
	}
	
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	vendor, exists := s.ouiMap[oui]
	if !exists {
		return ""
	}
	
	return vendor
}

// extractOUI extracts and normalizes the OUI from a MAC address
func (s *OUIService) extractOUI(macAddress string) string {
	// Remove common MAC address separators
	mac := strings.ReplaceAll(macAddress, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	mac = strings.ReplaceAll(mac, ".", "")
	mac = strings.ToUpper(mac)
	
	// Ensure we have at least 6 characters for OUI
	if len(mac) < 6 {
		return ""
	}
	
	return mac[:6]
}

// GetStatistics returns statistics about the loaded OUI database
func (s *OUIService) GetStatistics() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return map[string]interface{}{
		"total_entries": len(s.ouiMap),
		"last_updated":  s.getLastUpdated(),
	}
}

// getLastUpdated returns the last update time of the OUI database file
func (s *OUIService) getLastUpdated() string {
	ouiFile := filepath.Join(s.dataPath, "oui.txt")
	info, err := os.Stat(ouiFile)
	if err != nil {
		return "unknown"
	}
	return info.ModTime().Format("2006-01-02 15:04:05")
}