package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/yohannes/chines-backend/internal/platform/httpx"
	"github.com/yohannes/chines-backend/internal/user"
)

type identityKey struct{}

type Identity struct {
	UserID string
	Role   user.Role
}

// IdentityFrom returns who is making the request (set by Authenticate).
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}

// Authenticate rejects requests without a valid access token.
func Authenticate(tm *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || raw == "" {
				httpx.Error(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			claims, err := tm.Parse(raw)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), identityKey{}, Identity{
				UserID: claims.Subject,
				Role:   claims.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole allows only the listed roles. Use it AFTER Authenticate.
func RequireRole(roles ...user.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFrom(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "not authenticated")
				return
			}
			for _, role := range roles {
				if id.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.Error(w, http.StatusForbidden, "you do not have permission")
		})
	}
}