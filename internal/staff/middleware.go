package staff

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/nevermore222/sangyu-record/internal/httpapi"
)

type Authenticator interface {
	Authenticate(context.Context, string) (Staff, error)
}

type Middleware struct {
	authenticator Authenticator
}

func NewMiddleware(authenticator Authenticator) *Middleware {
	return &Middleware{authenticator: authenticator}
}

type contextKey struct{}

func (m *Middleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			writeAuthError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
			return
		}
		value, err := m.authenticator.Authenticate(r.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, ErrForbidden) {
				status = http.StatusForbidden
			}
			writeAuthError(w, status, "staff_unauthorized", "staff session is invalid")
			return
		}
		ctx := context.WithValue(r.Context(), contextKey{}, value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func FromContext(ctx context.Context) (Staff, bool) {
	value, ok := ctx.Value(contextKey{}).(Staff)
	return value, ok
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
