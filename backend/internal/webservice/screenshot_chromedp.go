//go:build chromedp

// Chromedp-backed screenshot capture. Built only with `-tags chromedp`;
// chromedp + cdproto pull in ~57 packages and roughly 7MB of binary, which
// is a lot to carry for a feature that already has four exec-based
// fallbacks. See screenshot_nochromedp.go for the default build.
package webservice

import (
	"context"
	"encoding/base64"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

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
