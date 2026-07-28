package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct{}

func NewRouter(_ Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return router
}
