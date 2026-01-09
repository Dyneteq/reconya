package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

// ServePage serves the main application page with the specified page context
func (h *WebHandler) ServePage(pageName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := h.sessionStore.Get(r, "reconya-session")
		user := h.getUserFromSession(session)
		if user == nil {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Map root path to dashboard
		if pageName == "" {
			pageName = "dashboard"
		}

		data := PageData{
			Page: pageName,
			User: user,
		}

		if err := h.templates.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("%s template execution error: %v", pageName, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Index serves the dashboard page
func (h *WebHandler) Index(w http.ResponseWriter, r *http.Request) {
	h.ServePage("dashboard")(w, r)
}

// Login handles login page and authentication
func (h *WebHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		loginTmpl, err := template.ParseFiles(filepath.Join(findPath("templates"), "standalone", "login.html"))
		if err != nil {
			log.Printf("Failed to parse standalone login template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Page     string
			Error    string
			Username string
		}{
			Page:     "login",
			Error:    "",
			Username: "",
		}
		if err := loginTmpl.Execute(w, data); err != nil {
			log.Printf("Template execution error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Handle POST login
	username := r.FormValue("username")
	password := r.FormValue("password")

	if h.authenticate(username, password) {
		session, _ := h.sessionStore.Get(r, "reconya-session")
		session.Values["user_id"] = username
		session.Values["username"] = username
		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	} else {
		loginTmpl, err := template.ParseFiles(filepath.Join(findPath("templates"), "standalone", "login.html"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Page     string
			Error    string
			Username string
		}{
			Page:     "login",
			Error:    "Invalid username or password",
			Username: username,
		}
		if err := loginTmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Logout handles user logout
func (h *WebHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.sessionStore.Get(r, "reconya-session")
	session.Values = make(map[interface{}]interface{})
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
