package server

import (
	_ "embed"
	"html/template"
	"net/http"
)

//go:embed templates/404.html
var raw404 string

//go:embed templates/500.html
var raw500 string

//go:embed templates/401.html
var raw401 string

var (
	tmpl404 = template.Must(template.New("404").Parse(raw404))
	tmpl500 = template.Must(template.New("500").Parse(raw500))
	tmpl401 = template.Must(template.New("401").Parse(raw401))
)

// serve404 writes a styled 404 Not Found HTML response.
func serve404(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	tmpl404.Execute(w, nil)
}

// serve500 writes a styled 500 Internal Server Error HTML response.
func serve500(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	tmpl500.Execute(w, nil)
}

// serve401 writes a styled 401 Unauthorized HTML response.
func serve401(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	tmpl401.Execute(w, nil)
}
