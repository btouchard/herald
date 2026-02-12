package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kolapsis/herald/internal/config"
)

// OAuthServer implements OAuth 2.1 with PKCE for Claude Chat Custom Connectors.
type OAuthServer struct {
	clientID     string
	clientSecret string
	publicURL    string
	secret       []byte // HMAC signing key derived from client secret
	redirectURIs []string

	accessTTL  time.Duration
	refreshTTL time.Duration

	store AuthStore
}

// NewOAuthServer creates an OAuth 2.1 server from config.
func NewOAuthServer(cfg config.AuthConfig, publicURL string) *OAuthServer {
	return NewOAuthServerWithStore(cfg, publicURL, NewMemoryAuthStore())
}

// NewOAuthServerWithStore creates an OAuth 2.1 server with the given auth store.
func NewOAuthServerWithStore(cfg config.AuthConfig, publicURL string, store AuthStore) *OAuthServer {
	secret := sha256.Sum256([]byte(cfg.ClientSecret))

	accessTTL := cfg.AccessTokenTTL
	if accessTTL == 0 {
		accessTTL = time.Hour
	}
	refreshTTL := cfg.RefreshTokenTTL
	if refreshTTL == 0 {
		refreshTTL = 30 * 24 * time.Hour
	}

	return &OAuthServer{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		publicURL:    strings.TrimRight(publicURL, "/"),
		secret:       secret[:],
		redirectURIs: cfg.RedirectURIs,
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		store:        store,
	}
}

// isValidRedirectURI checks the URI against the configured allowlist (exact match).
func (s *OAuthServer) isValidRedirectURI(uri string) bool {
	for _, allowed := range s.redirectURIs {
		if uri == allowed {
			return true
		}
	}
	return false
}

// Secret returns the HMAC signing key (for middleware use).
func (s *OAuthServer) Secret() []byte {
	return s.secret
}

// HandleMetadata serves the OAuth 2.1 server metadata (RFC 8414).
// GET /.well-known/oauth-authorization-server
func (s *OAuthServer) HandleMetadata(w http.ResponseWriter, r *http.Request) {
	metadata := map[string]interface{}{
		"issuer":                                s.publicURL,
		"authorization_endpoint":                s.publicURL + "/oauth/authorize",
		"token_endpoint":                        s.publicURL + "/oauth/token",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metadata)
}

// HandleProtectedResourceMetadata serves the Protected Resource Metadata (RFC 9728).
// GET /.well-known/oauth-protected-resource
func (s *OAuthServer) HandleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	metadata := map[string]interface{}{
		"resource":                s.publicURL + "/mcp",
		"authorization_servers":   []string{s.publicURL},
		"bearer_methods_supported": []string{"header"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metadata)
}

// HandleAuthorize handles the authorization endpoint.
// GET /oauth/authorize
func (s *OAuthServer) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	responseType := q.Get("response_type")
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")
	state := q.Get("state")
	scope := q.Get("scope")

	// Validate redirect_uri BEFORE any redirect to prevent open redirect.
	// If invalid, respond with HTTP 400 JSON — never redirect to an untrusted URI.
	if redirectURI == "" || !s.isValidRedirectURI(redirectURI) {
		slog.Warn("authorization request with invalid redirect_uri",
			"redirect_uri", redirectURI,
			"client_id", clientID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_request",
			"error_description": "redirect_uri is missing or not registered",
		})
		return
	}

	if responseType != "code" {
		oauthError(w, r, redirectURI, state, "unsupported_response_type", "only 'code' is supported")
		return
	}

	if clientID != s.clientID {
		oauthError(w, r, redirectURI, state, "invalid_client", "unknown client_id")
		return
	}

	// PKCE is mandatory per OAuth 2.1 — reject requests without code_challenge.
	if codeChallenge == "" {
		oauthError(w, r, redirectURI, state, "invalid_request", "code_challenge is required (PKCE)")
		return
	}

	if codeChallengeMethod != "S256" {
		oauthError(w, r, redirectURI, state, "invalid_request", "code_challenge_method must be S256")
		return
	}

	// For MVP: auto-approve (no login page yet).
	// Generate authorization code and redirect.
	code, codeHash := generateCode()

	s.store.StoreCode(&AuthCode{
		CodeHash:      codeHash,
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		CodeChallenge: codeChallenge,
		Scope:         scope,
		ExpiresAt:     time.Now().Add(10 * time.Minute),
	})

	slog.Info("authorization code issued",
		"client_id", clientID,
		"scope", scope)

	redirectURL := buildRedirect(redirectURI, map[string]string{
		"code":  code,
		"state": state,
	})

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleToken handles the token endpoint.
// POST /oauth/token
func (s *OAuthServer) HandleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		tokenError(w, http.StatusBadRequest, "invalid_request", "malformed form data")
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		s.handleAuthorizationCode(w, r)
	case "refresh_token":
		s.handleRefreshToken(w, r)
	default:
		tokenError(w, http.StatusBadRequest, "unsupported_grant_type", "")
	}
}

func (s *OAuthServer) handleAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")

	if clientID != s.clientID {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "")
		return
	}

	if clientSecret != s.clientSecret {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "")
		return
	}

	codeHash := HashToken(code)
	authCode, err := s.store.ConsumeCode(codeHash)
	if err != nil {
		tokenError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}

	if authCode.ClientID != clientID {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}

	// Verify redirect_uri matches the one used during authorization
	if redirectURI != authCode.RedirectURI {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}

	// PKCE verification — always enforced, never optional.
	if codeVerifier == "" {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "code_verifier is required (PKCE)")
		return
	}
	if !verifyPKCE(codeVerifier, authCode.CodeChallenge) {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	s.issueTokenPair(w, clientID, authCode.Scope)
}

func (s *OAuthServer) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")

	if clientID != s.clientID || clientSecret != s.clientSecret {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "")
		return
	}

	// Verify the refresh token JWT
	claims, err := VerifyToken(refreshToken, s.secret)
	if err != nil {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "invalid refresh token")
		return
	}

	if claims.TokenType != "refresh" {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "not a refresh token")
		return
	}

	// Check the token is stored and not revoked
	tokenHash := HashToken(refreshToken)
	if _, err := s.store.GetToken(tokenHash); err != nil {
		tokenError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}

	// Revoke old refresh token (rotation)
	s.store.RevokeToken(tokenHash)

	s.issueTokenPair(w, clientID, claims.Scope)
}

func (s *OAuthServer) issueTokenPair(w http.ResponseWriter, clientID, scope string) {
	now := time.Now()

	accessClaims := TokenClaims{
		Subject:   "herald-user",
		ClientID:  clientID,
		Scope:     scope,
		TokenType: "access",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.accessTTL).Unix(),
		Issuer:    s.publicURL,
	}

	accessToken, err := SignToken(accessClaims, s.secret)
	if err != nil {
		tokenError(w, http.StatusInternalServerError, "server_error", "failed to sign access token")
		return
	}

	refreshClaims := TokenClaims{
		Subject:   "herald-user",
		ClientID:  clientID,
		Scope:     scope,
		TokenType: "refresh",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.refreshTTL).Unix(),
		Issuer:    s.publicURL,
	}

	refreshToken, err := SignToken(refreshClaims, s.secret)
	if err != nil {
		tokenError(w, http.StatusInternalServerError, "server_error", "failed to sign refresh token")
		return
	}

	// Store tokens
	s.store.StoreToken(&StoredToken{
		TokenHash: HashToken(accessToken),
		TokenType: "access",
		ClientID:  clientID,
		Scope:     scope,
		ExpiresAt: now.Add(s.accessTTL),
	})

	s.store.StoreToken(&StoredToken{
		TokenHash: HashToken(refreshToken),
		TokenType: "refresh",
		ClientID:  clientID,
		Scope:     scope,
		ExpiresAt: now.Add(s.refreshTTL),
	})

	slog.Info("tokens issued", "client_id", clientID, "scope", scope)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    int(s.accessTTL.Seconds()),
		"refresh_token": refreshToken,
		"scope":         scope,
	})
}

// verifyPKCE checks that SHA256(code_verifier) == code_challenge.
func verifyPKCE(codeVerifier, codeChallenge string) bool {
	h := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])
	return computed == codeChallenge
}

func generateCode() (raw string, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read always succeeds on supported platforms
	raw = hex.EncodeToString(b)
	hash = HashToken(raw)
	return
}

func buildRedirect(baseURI string, params map[string]string) string {
	u, err := url.Parse(baseURI)
	if err != nil {
		return baseURI
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func oauthError(w http.ResponseWriter, r *http.Request, redirectURI, state, errCode, desc string) {
	if redirectURI != "" {
		u := buildRedirect(redirectURI, map[string]string{
			"error":             errCode,
			"error_description": desc,
			"state":             state,
		})
		http.Redirect(w, r, u, http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             errCode,
		"error_description": desc,
	})
}

func tokenError(w http.ResponseWriter, status int, errCode, desc string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]string{"error": errCode}
	if desc != "" {
		resp["error_description"] = desc
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// StartCleanupLoop runs periodic cleanup of expired tokens and codes.
func (s *OAuthServer) StartCleanupLoop(done <-chan struct{}) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.store.Cleanup()
		case <-done:
			return
		}
	}
}

// ValidateAccessToken verifies an access token string (for middleware).
func (s *OAuthServer) ValidateAccessToken(tokenStr string) (*TokenClaims, error) {
	claims, err := VerifyToken(tokenStr, s.secret)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "access" {
		return nil, fmt.Errorf("not an access token")
	}

	tokenHash := HashToken(tokenStr)
	if _, err := s.store.GetToken(tokenHash); err != nil {
		return nil, fmt.Errorf("token not recognized: %w", err)
	}

	return claims, nil
}
