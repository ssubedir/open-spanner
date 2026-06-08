package adminui

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
		http.Redirect(w, r, "/admin", http.StatusFound)
	})
	router.Handle("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently))
	router.Handle("/admin/*", http.StripPrefix("/admin/", http.FileServer(http.FS(staticFiles))))
}
