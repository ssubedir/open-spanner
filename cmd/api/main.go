package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"open-spanner/internal/adminui"
	"open-spanner/internal/metering/bootstrap"
)

func main() {
	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	adminui.RegisterRoutes(router)
	bootstrap.RegisterRoutes(router)

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
