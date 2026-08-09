package webservice

import "testing"

// captureScreenshot treats an empty string from any backend as "this one is
// unavailable, try the next". The default (no-tag) build's captureWithChromedp
// is a no-op returning "", so this asserts the contract the fallback chain
// relies on rather than the chromedp behaviour itself — it must hold in both
// build variants for the chain to degrade instead of breaking.
func TestCaptureWithChromedpHonoursEmptyIsUnavailable(t *testing.T) {
	w := NewWebService()

	// An unroutable address: no backend can succeed, so the whole chain must
	// fall through and return "" rather than panicking or hanging forever.
	got := w.captureWithChromedp("http://127.0.0.1:1/")
	if got != "" {
		t.Errorf("captureWithChromedp on an unreachable URL = %q, want \"\" so the fallback chain continues", got)
	}
}
