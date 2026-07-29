package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	RegisterStaffRoutes    func(chi.Router)
	RegisterProviderRoutes func(chi.Router)
}

func NewRouter(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if deps.RegisterStaffRoutes != nil {
		router.Route("/v1/staff", deps.RegisterStaffRoutes)
	}
	if deps.RegisterProviderRoutes != nil {
		deps.RegisterProviderRoutes(router)
	}
	return router
}
