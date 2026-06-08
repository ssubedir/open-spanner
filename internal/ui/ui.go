package ui

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed static
var files embed.FS

func RegisterRoutes(router chi.Router) {
	staticFiles, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/overview", http.StatusFound)
	})
	router.Get("/overview", servePage(staticFiles, "index.html"))
	router.Get("/meters", servePage(staticFiles, "index.html"))
	router.Get("/usage", servePage(staticFiles, "index.html"))
	router.Get("/favicon.svg", serveFile(staticFiles, "favicon.svg"))
	router.Get("/icons.svg", serveFile(staticFiles, "icons.svg"))
	router.Handle("/assets/*", http.StripPrefix("/", http.FileServer(http.FS(staticFiles))))
}

func servePage(files fs.FS, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := fs.ReadFile(files, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	}
}

func serveFile(files fs.FS, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, files, name)
	}
}
