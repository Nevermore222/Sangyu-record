package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	Projects http.Handler
}

func NewRouter(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if deps.Projects != nil {
		router.Mount("/v1/staff/projects", deps.Projects)
	}
	return router
}
