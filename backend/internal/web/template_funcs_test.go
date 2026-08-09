package web

import (
	"bytes"
	"html/template"
	"testing"

	"reconya/models"
)

// The template FuncMap used to override the "or" and "eq" builtins with
// implementations that were wrong for the way the templates actually call
// them. These tests pin the builtin behaviour so a future helper can't
// silently shadow them again.
//
// The old "or" returned the first argument that was neither nil nor "";
// given booleans that is the first argument whenever it is false, so a
// multi-branch page check collapsed to "is it the first page?".
func TestOrIsLogicalNotCoalesce(t *testing.T) {
	tmpl := template.Must(template.New("t").Funcs(templateFuncMap()).Parse(
		`{{if or (eq .Page "devices") (eq .Page "networks") (eq .Page "logs")}}YES{{else}}NO{{end}}`))

	for _, tc := range []struct {
		page string
		want string
	}{
		{"devices", "YES"},  // first arg true — worked even with the old "or"
		{"networks", "YES"}, // middle arg true — regressed with the old "or"
		{"logs", "YES"},     // last arg true — regressed with the old "or"
		{"dashboard", "NO"},
	} {
		var b bytes.Buffer
		if err := tmpl.Execute(&b, struct{ Page string }{tc.page}); err != nil {
			t.Fatalf("page %q: %v", tc.page, err)
		}
		if got := b.String(); got != tc.want {
			t.Errorf("page %q: got %q, want %q", tc.page, got, tc.want)
		}
	}
}

// The old "eq" was `a == b` over interface{}, which compares dynamic type as
// well as value. models.DeviceStatus is a named string type, so comparing one
// against an untyped string literal was always false.
func TestEqComparesNamedStringTypesByValue(t *testing.T) {
	tmpl := template.Must(template.New("t").Funcs(templateFuncMap()).Parse(
		`{{if eq .Status "online"}}ONLINE{{else}}NOT{{end}}`))

	var b bytes.Buffer
	if err := tmpl.Execute(&b, struct{ Status models.DeviceStatus }{models.DeviceStatusOnline}); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != "ONLINE" {
		t.Errorf("eq on models.DeviceStatus: got %q, want %q", got, "ONLINE")
	}
}
