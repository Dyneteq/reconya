package reconya_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"reconya/internal/config"
	"reconya/internal/web/handlers"
)

// TestSensorsRouteRendersSensorPage ensures GET /sensors renders the sensor page
func TestSensorsRouteRendersSensorPage(t *testing.T) {
	handler := handlers.NewWebHandler(
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		&config.Config{},
		"test-secret",
	)
	router := handler.SetupRoutes()

	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", "password")

	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusSeeOther {
		t.Fatalf("expected login redirect, got status %d", loginRec.Code)
	}

	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie after login")
	}

	req := httptest.NewRequest("GET", "/sensors", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected sensors page 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "[ SENSORS ]") {
		t.Fatalf("expected sensors page content, got: %s", rec.Body.String())
	}
}
