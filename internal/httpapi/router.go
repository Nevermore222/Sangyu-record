package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	RegisterAuthRoutes     func(chi.Router)
	RegisterStaffRoutes    func(chi.Router)
	RegisterProviderRoutes func(chi.Router)
	StaffMiddleware        func(http.Handler) http.Handler
}

func NewRouter(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if deps.RegisterAuthRoutes != nil {
		deps.RegisterAuthRoutes(router)
	}
	if deps.RegisterStaffRoutes != nil {
		router.Route("/v1/staff", func(staffRouter chi.Router) {
			if deps.StaffMiddleware != nil {
				staffRouter.Use(deps.StaffMiddleware)
			}
			deps.RegisterStaffRoutes(staffRouter)
		})
	}
	if deps.RegisterProviderRoutes != nil {
		deps.RegisterProviderRoutes(router)
	}
	return router
}
