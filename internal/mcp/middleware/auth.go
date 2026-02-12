package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/kolapsis/herald/internal/auth"
)

// BearerAuth returns middleware that validates OAuth 2.1 Bearer tokens.
// The resourceMetadataURL is included in the WWW-Authenticate header per RFC 9728.
func BearerAuth(oauth *auth.OAuthServer, resourceMetadataURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				challengeAuth(w, resourceMetadataURL, "missing Authorization header")
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				challengeAuth(w, resourceMetadataURL, "invalid Authorization header format")
				return
			}

			token := parts[1]
			_, err := oauth.ValidateAccessToken(token)
			if err != nil {
				slog.Debug("token validation failed", "error", err)
				invalidToken(w, "invalid or expired token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// challengeAuth sends a 401 with a Bearer challenge for unauthenticated requests.
// Includes resource_metadata URL per MCP spec (RFC 9728) so clients can discover the auth server.
func challengeAuth(w http.ResponseWriter, resourceMetadataURL, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+resourceMetadataURL+`"`)
	http.Error(w, msg, http.StatusUnauthorized)
}

// invalidToken sends a 401 for requests with an invalid/expired Bearer token.
func invalidToken(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	http.Error(w, msg, http.StatusUnauthorized)
}
