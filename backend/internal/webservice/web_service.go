package webservice

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reconya/models"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type WebService struct {
	client             *http.Client
	screenshotsEnabled bool
}

type WebInfo struct {
	URL         string
	Title       string
	Server      string
	StatusCode  int
	ContentType string
	Size        int64
	Screenshot  string // Base64 encoded screenshot or file path
}

func NewWebService() *WebService {
	// Create HTTP client with timeouts and insecure TLS (for self-signed certs)
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	return &WebService{
		client: client,
	}
}

// ScanWebServices checks for HTTP/HTTPS services on a device and extracts web info
func (w *WebService) ScanWebServices(device *models.Device) []WebInfo {
	return w.ScanWebServicesWithScreenshots(device, false)
}

// ScanWebServicesWithScreenshots checks for HTTP/HTTPS services on a device and optionally captures screenshots
func (w *WebService) ScanWebServicesWithScreenshots(device *models.Device, captureScreenshots bool) []WebInfo {
	var webInfos []WebInfo

	if device.Ports == nil || len(device.Ports) == 0 {
		return webInfos
	}

	// Common web ports to check
	webPorts := map[string][]string{
		"80":   {"http"},
		"443":  {"https"},
		"8080": {"http"},
		"8443": {"https"},
		"8000": {"http"},
		"8008": {"http"},
		"8081": {"http"},
		"9000": {"http"},
		"3000": {"http"},
		"5000": {"http"},
	}

	// Check each port to see if it's a web service
	for _, port := range device.Ports {
		if port.State != "open" {
			continue
		}

		protocols, isWebPort := webPorts[port.Number]
		if !isWebPort {
			// Also check if service name indicates web service
			serviceName := strings.ToLower(port.Service)
			if strings.Contains(serviceName, "http") || strings.Contains(serviceName, "web") {
				// Guess protocol based on port number
				if port.Number == "443" || port.Number == "8443" {
					protocols = []string{"https"}
				} else {
					protocols = []string{"http"}
				}
			} else {
				continue
			}
		}

		// Convert port number to int for fetchWebInfo
		portNum, err := strconv.Atoi(port.Number)
		if err != nil {
			log.Printf("Invalid port number: %s", port.Number)
			continue
		}

		// Try each protocol for this port
		for _, protocol := range protocols {
			webInfo := w.fetchWebInfo(device.IPv4, portNum, protocol, captureScreenshots)
			if webInfo != nil {
				webInfos = append(webInfos, *webInfo)
				log.Printf("Found web service: %s on %s:%s", protocol, device.IPv4, port.Number)
			}
		}
	}

	return webInfos
}

// fetchWebInfo attempts to fetch web page info from a specific URL
func (w *WebService) fetchWebInfo(ip string, port int, protocol string, captureScreenshots bool) *WebInfo {
	urlStr := fmt.Sprintf("%s://%s:%d", protocol, ip, port)
	
	log.Printf("Fetching web info from: %s", urlStr)
	
	resp, err := w.client.Get(urlStr)
	if err != nil {
		log.Printf("Failed to fetch %s: %v", urlStr, err)
		return nil
	}
	defer resp.Body.Close()

	// Read response body (limit to 1MB to avoid memory issues)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		log.Printf("Failed to read response body from %s: %v", urlStr, err)
		return nil
	}

	webInfo := &WebInfo{
		URL:         urlStr,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Server:      resp.Header.Get("Server"),
		Size:        int64(len(body)),
	}

	// Extract title from HTML
	if strings.Contains(strings.ToLower(webInfo.ContentType), "html") {
		webInfo.Title = w.extractTitle(string(body))
	}

	// Only consider successful responses
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		// Capture screenshot for successful web pages only if requested
		if captureScreenshots && strings.Contains(strings.ToLower(webInfo.ContentType), "html") {
			log.Printf("Attempting to capture screenshot for %s", urlStr)
			screenshot := w.captureScreenshot(urlStr)
			if screenshot != "" {
				webInfo.Screenshot = screenshot
				log.Printf("Successfully captured screenshot for %s (size: %d bytes)", urlStr, len(screenshot))
			} else {
				log.Printf("Failed to capture screenshot for %s", urlStr)
			}
		}
		return webInfo
	}

	return nil
}

// extractTitle extracts the title from HTML content
func (w *WebService) extractTitle(html string) string {
	// Use regex to find title tag (case insensitive)
	titleRegex := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	matches := titleRegex.FindStringSubmatch(html)
	
	if len(matches) > 1 {
		title := strings.TrimSpace(matches[1])
		// Clean up title (remove extra whitespace, newlines)
		title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
		return title
	}

	return ""
}

// GetCommonWebPorts returns a list of common web ports to check
func (w *WebService) GetCommonWebPorts() []int {
	return []int{80, 443, 8080, 8443, 8000, 8008, 8081, 9000, 3000, 5000}
}

// IsWebPort checks if a port number is commonly used for web services
func (w *WebService) IsWebPort(port string) bool {
	webPorts := map[string]bool{
		"80": true, "443": true, "8080": true, "8443": true,
		"8000": true, "8008": true, "8081": true, "9000": true,
		"3000": true, "5000": true,
	}
	return webPorts[port]
}

// ValidateURL checks if a URL is valid and reachable
func (w *WebService) ValidateURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}

	resp, err := w.client.Head(urlStr)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// captureScreenshot captures a screenshot of a web page and returns base64 encoded image
func (w *WebService) captureScreenshot(urlStr string) string {
	// Create a temporary directory for screenshots
	tempDir := "/tmp/reconya-screenshots"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("Failed to create screenshot directory: %v", err)
		return ""
	}

	// Generate a unique filename
	filename := fmt.Sprintf("screenshot_%d.png", time.Now().UnixNano())
	screenshotPath := filepath.Join(tempDir, filename)

	// Try chromedp (Go-based, no external dependencies) first
	screenshot := w.captureWithChromedp(urlStr)
	if screenshot != "" {
		return screenshot
	}

	// Try Firefox ESR headless if available
	screenshot = w.captureWithFirefox(urlStr, screenshotPath)
	if screenshot != "" {
		return screenshot
	}

	// Try Chrome/Chromium headless if available
	screenshot = w.captureWithChrome(urlStr, screenshotPath)
	if screenshot != "" {
		return screenshot
	}

	// Fallback to other methods if Chrome is not available
	screenshot = w.captureWithWkhtmltoimage(urlStr, screenshotPath)
	if screenshot != "" {
		return screenshot
	}

	// Try webkit2png if available (macOS)
	screenshot = w.captureWithWebkit2png(urlStr, screenshotPath)
	if screenshot != "" {
		return screenshot
	}

	log.Printf("No screenshot method available for %s", urlStr)
	return ""
}

// captureWithChromedp captures screenshot using chromedp (Go-based, no external dependencies)
func (w *WebService) captureWithChromedp(urlStr string) string {
	log.Printf("Attempting chromedp screenshot for %s", urlStr)
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create chromedp context with options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-plugins", true),
		chromedp.WindowSize(1280, 1024),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	// Create chrome instance
	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	// Capture screenshot
	var screenshotData []byte
	err := chromedp.Run(taskCtx,
		chromedp.Navigate(urlStr),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for content to load
		chromedp.CaptureScreenshot(&screenshotData),
	)

	if err != nil {
		log.Printf("chromedp screenshot failed for %s: %v", urlStr, err)
		return ""
	}

	if len(screenshotData) == 0 {
		log.Printf("chromedp returned empty screenshot for %s", urlStr)
		return ""
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(screenshotData)
	log.Printf("chromedp screenshot successful for %s (size: %d bytes)", urlStr, len(screenshotData))
	
	return encoded
}

// captureWithChrome captures screenshot using Chrome/Chromium headless
func (w *WebService) captureWithChrome(urlStr, outputPath string) string {
	// Try different Chrome binary names and paths
	chromeBinaries := []string{
		"google-chrome",
		"chromium", 
		"chromium-browser", 
		"chrome",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", // macOS
		"/usr/bin/google-chrome",     // Linux
		"/usr/bin/chromium-browser",  // Linux
	}
	
	var chromeCmd string
	for _, binary := range chromeBinaries {
		if _, err := exec.LookPath(binary); err == nil {
			chromeCmd = binary
			break
		}
		// Also check if file exists directly (for full paths)
		if _, err := os.Stat(binary); err == nil {
			chromeCmd = binary
			break
		}
	}
	
	if chromeCmd == "" {
		log.Printf("Chrome/Chromium not found in PATH or standard locations")
		return ""
	}
	
	log.Printf("Using Chrome binary: %s", chromeCmd)

	// Chrome headless command with security options for containers
	args := []string{
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--disable-dev-shm-usage",
		"--disable-extensions",
		"--disable-plugins",
		"--window-size=1280,1024",
		"--timeout=10000",
		fmt.Sprintf("--screenshot=%s", outputPath),
		urlStr,
	}

	cmd := exec.Command(chromeCmd, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	
	// Set timeout for screenshot capture
	timer := time.AfterFunc(15*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	err := cmd.Run()
	if err != nil {
		log.Printf("Chrome screenshot failed for %s: %v", urlStr, err)
		return ""
	}

	return w.encodeScreenshotToBase64(outputPath)
}

// captureWithFirefox captures screenshot using Firefox ESR headless
func (w *WebService) captureWithFirefox(urlStr, outputPath string) string {
	// Check if firefox is available
	if _, err := exec.LookPath("firefox"); err != nil {
		log.Printf("Firefox not found in PATH")
		return ""
	}

	log.Printf("Using Firefox ESR for screenshot")

	// Firefox headless command with screenshot
	args := []string{
		"--headless",
		"--screenshot",
		outputPath,
		"--window-size=1280,1024",
		urlStr,
	}

	// Use xvfb-run to provide virtual display for Firefox
	cmd := exec.Command("xvfb-run", append([]string{"-a", "firefox"}, args...)...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	
	// Set timeout for screenshot capture
	timer := time.AfterFunc(15*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	err := cmd.Run()
	if err != nil {
		log.Printf("Firefox screenshot failed for %s: %v", urlStr, err)
		return ""
	}

	return w.encodeScreenshotToBase64(outputPath)
}

// captureWithWkhtmltoimage captures screenshot using wkhtmltoimage
func (w *WebService) captureWithWkhtmltoimage(urlStr, outputPath string) string {
	// Check if wkhtmltoimage is available
	if _, err := exec.LookPath("wkhtmltoimage"); err != nil {
		log.Printf("wkhtmltoimage not found in PATH")
		return ""
	}

	args := []string{
		"--width", "1280",
		"--height", "1024",
		"--javascript-delay", "2000",
		"--load-error-handling", "ignore",
		"--load-media-error-handling", "ignore",
		urlStr,
		outputPath,
	}

	cmd := exec.Command("wkhtmltoimage", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	
	// Set timeout for screenshot capture
	timer := time.AfterFunc(15*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	err := cmd.Run()
	if err != nil {
		log.Printf("wkhtmltoimage screenshot failed for %s: %v", urlStr, err)
		return ""
	}

	return w.encodeScreenshotToBase64(outputPath)
}

// captureWithWebkit2png captures screenshot using webkit2png (macOS)
func (w *WebService) captureWithWebkit2png(urlStr, outputPath string) string {
	// Check if webkit2png is available
	if _, err := exec.LookPath("webkit2png"); err != nil {
		log.Printf("webkit2png not found in PATH")
		return ""
	}

	// webkit2png saves as .png by default, so we need to adjust the output path
	baseOutputPath := strings.TrimSuffix(outputPath, ".png")
	
	args := []string{
		"--width=1280",
		"--height=1024",
		"--delay=2",
		"--no-print-backgrounds",
		fmt.Sprintf("--dir=%s", filepath.Dir(outputPath)),
		fmt.Sprintf("--filename=%s", filepath.Base(baseOutputPath)),
		urlStr,
	}

	cmd := exec.Command("webkit2png", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	
	// Set timeout for screenshot capture
	timer := time.AfterFunc(15*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	err := cmd.Run()
	if err != nil {
		log.Printf("webkit2png screenshot failed for %s: %v", urlStr, err)
		return ""
	}

	// webkit2png creates a file with -full.png suffix
	actualOutputPath := fmt.Sprintf("%s-full.png", baseOutputPath)
	
	// Check if the file was created
	if _, err := os.Stat(actualOutputPath); os.IsNotExist(err) {
		log.Printf("webkit2png did not create expected file: %s", actualOutputPath)
		return ""
	}

	return w.encodeScreenshotToBase64(actualOutputPath)
}

// encodeScreenshotToBase64 reads a screenshot file and encodes it as base64
func (w *WebService) encodeScreenshotToBase64(filePath string) string {
	// Read the screenshot file
	imageData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Failed to read screenshot file %s: %v", filePath, err)
		return ""
	}

	// Clean up the file
	defer func() {
		if err := os.Remove(filePath); err != nil {
			log.Printf("Failed to remove screenshot file %s: %v", filePath, err)
		}
	}()

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(imageData)
	log.Printf("Screenshot encoded to base64, size: %d bytes", len(imageData))
	
	return encoded
}