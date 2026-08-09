//go:build !chromedp

// Default build: no chromedp. Screenshot capture falls through to the
// exec-based backends (Chrome, Firefox, wkhtmltoimage, webkit2png), which
// need no Go dependencies at all. Build with `-tags chromedp` to link in
// the in-process chromedp driver instead.
package webservice

// captureWithChromedp is a no-op in the default build; captureScreenshot
// treats an empty return as "this backend is unavailable" and moves on to
// the next one.
func (w *WebService) captureWithChromedp(urlStr string) string {
	return ""
}
