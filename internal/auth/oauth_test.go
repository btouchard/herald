package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kolapsis/herald/internal/config"
)

const testRedirectURI = "https://callback.test/cb"

// testPKCE holds a verifier/challenge pair for tests.
type testPKCE struct {
	Verifier  string
	Challenge string
}

// newTestPKCE returns a deterministic PKCE pair for test reproducibility.
func newTestPKCE() testPKCE {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	return testPKCE{
		Verifier:  verifier,
		Challenge: computeS256Challenge(verifier),
	}
}

func newTestOAuth() *OAuthServer {
	return NewOAuthServer(config.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{testRedirectURI},
	}, "https://herald.test")
}

func TestHandleMetadata_ReturnsCorrectEndpoints(t *testing.T) {
	t.Parallel()

	oauth := newTestOAuth()
	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	oauth.HandleMetadata(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &meta))

	assert.Equal(t, "https://herald.test", meta["issuer"])
	assert.Equal(t, "https://herald.test/oauth/authorize", meta["authorization_endpoint"])
	assert.Equal(t, "https://herald.test/oauth/token", meta["token_endpoint"])
}

func TestOAuthFlow_AuthorizeAndToken(t *testing.T) {
	oauth := newTestOAuth()
	pkce := newTestPKCE()

	// Step 1: GET /oauth/authorize
	authReq := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {"https://callback.test/cb"},
		"state":                 {"xyz"},
		"scope":                 {"tasks"},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	authW := httptest.NewRecorder()

	oauth.HandleAuthorize(authW, authReq)

	assert.Equal(t, http.StatusFound, authW.Code)

	location := authW.Header().Get("Location")
	require.NotEmpty(t, location)

	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	assert.Equal(t, "xyz", redirectURL.Query().Get("state"))

	code := redirectURL.Query().Get("code")
	require.NotEmpty(t, code, "authorization code should be present")

	// Step 2: POST /oauth/token (exchange code for tokens)
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
		"redirect_uri":  {testRedirectURI},
		"code_verifier": {pkce.Verifier},
	}
	tokenReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenW := httptest.NewRecorder()

	oauth.HandleToken(tokenW, tokenReq)

	assert.Equal(t, http.StatusOK, tokenW.Code)

	var tokenResp map[string]interface{}
	require.NoError(t, json.Unmarshal(tokenW.Body.Bytes(), &tokenResp))

	assert.NotEmpty(t, tokenResp["access_token"])
	assert.NotEmpty(t, tokenResp["refresh_token"])
	assert.Equal(t, "Bearer", tokenResp["token_type"])
	assert.Equal(t, "tasks", tokenResp["scope"])

	// Step 3: Validate the access token
	accessToken := tokenResp["access_token"].(string)
	claims, err := oauth.ValidateAccessToken(accessToken)
	require.NoError(t, err)
	assert.Equal(t, "test-client", claims.ClientID)
	assert.Equal(t, "access", claims.TokenType)
}

func TestOAuthFlow_RefreshToken(t *testing.T) {
	oauth := newTestOAuth()
	pkce := newTestPKCE()

	// Get initial tokens
	authReq := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {testRedirectURI},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	authW := httptest.NewRecorder()
	oauth.HandleAuthorize(authW, authReq)

	redirectURL, _ := url.Parse(authW.Header().Get("Location"))
	code := redirectURL.Query().Get("code")

	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
		"redirect_uri":  {testRedirectURI},
		"code_verifier": {pkce.Verifier},
	}
	tokenReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenW := httptest.NewRecorder()
	oauth.HandleToken(tokenW, tokenReq)

	var firstResp map[string]interface{}
	require.NoError(t, json.Unmarshal(tokenW.Body.Bytes(), &firstResp))
	refreshToken := firstResp["refresh_token"].(string)

	// Use refresh token to get new tokens
	refreshForm := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
	}
	refreshReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(refreshForm.Encode()))
	refreshReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	refreshW := httptest.NewRecorder()
	oauth.HandleToken(refreshW, refreshReq)

	assert.Equal(t, http.StatusOK, refreshW.Code)

	var secondResp map[string]interface{}
	require.NoError(t, json.Unmarshal(refreshW.Body.Bytes(), &secondResp))
	assert.NotEmpty(t, secondResp["access_token"])
	assert.NotEmpty(t, secondResp["refresh_token"])

	// Old refresh token should be revoked (rotation)
	reuseForm := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
	}
	reuseReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(reuseForm.Encode()))
	reuseReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reuseW := httptest.NewRecorder()
	oauth.HandleToken(reuseW, reuseReq)

	assert.Equal(t, http.StatusBadRequest, reuseW.Code)
}

func TestOAuthFlow_PKCE(t *testing.T) {
	oauth := newTestOAuth()

	// Generate PKCE pair
	codeVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := computeS256Challenge(codeVerifier)

	// Authorize with code_challenge
	authReq := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {testRedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	authW := httptest.NewRecorder()
	oauth.HandleAuthorize(authW, authReq)

	redirectURL, _ := url.Parse(authW.Header().Get("Location"))
	code := redirectURL.Query().Get("code")

	// Exchange with correct code_verifier
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
		"redirect_uri":  {testRedirectURI},
		"code_verifier": {codeVerifier},
	}
	tokenReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenW := httptest.NewRecorder()
	oauth.HandleToken(tokenW, tokenReq)

	assert.Equal(t, http.StatusOK, tokenW.Code)
}

func TestOAuthFlow_PKCE_WrongVerifier(t *testing.T) {
	oauth := newTestOAuth()

	codeVerifier := "correct-verifier-value"
	challenge := computeS256Challenge(codeVerifier)

	authReq := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {testRedirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	authW := httptest.NewRecorder()
	oauth.HandleAuthorize(authW, authReq)

	redirectURL, _ := url.Parse(authW.Header().Get("Location"))
	code := redirectURL.Query().Get("code")

	// Exchange with WRONG code_verifier
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
		"redirect_uri":  {testRedirectURI},
		"code_verifier": {"wrong-verifier"},
	}
	tokenReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenW := httptest.NewRecorder()
	oauth.HandleToken(tokenW, tokenReq)

	assert.Equal(t, http.StatusBadRequest, tokenW.Code)

	var errResp map[string]string
	require.NoError(t, json.Unmarshal(tokenW.Body.Bytes(), &errResp))
	assert.Contains(t, errResp["error_description"], "PKCE")
}

func TestOAuthFlow_RejectsInvalidClient(t *testing.T) {
	oauth := newTestOAuth()

	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"some-code"},
		"client_id":     {"wrong-client"},
		"client_secret": {"wrong-secret"},
	}
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tokenForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	oauth.HandleToken(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOAuthFlow_RejectsCodeReuse(t *testing.T) {
	oauth := newTestOAuth()
	pkce := newTestPKCE()

	// Get a code
	authReq := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {testRedirectURI},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	authW := httptest.NewRecorder()
	oauth.HandleAuthorize(authW, authReq)

	redirectURL, _ := url.Parse(authW.Header().Get("Location"))
	code := redirectURL.Query().Get("code")

	// First use — should succeed
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
		"redirect_uri":  {testRedirectURI},
		"code_verifier": {pkce.Verifier},
	}
	req1 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w1 := httptest.NewRecorder()
	oauth.HandleToken(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second use — should fail
	req2 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	oauth.HandleToken(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestHandleAuthorize_WhenUnregisteredRedirectURI_Returns400(t *testing.T) {
	t.Parallel()

	oauth := newTestOAuth()

	req := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type": {"code"},
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://evil.com/steal"},
		"state":         {"abc"},
	}.Encode(), nil)
	w := httptest.NewRecorder()

	oauth.HandleAuthorize(w, req)

	// Must be HTTP 400 JSON, NOT a redirect to the evil URI
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, w.Header().Get("Location"), "must not redirect to unregistered URI")

	var errResp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_request", errResp["error"])
	assert.Contains(t, errResp["error_description"], "redirect_uri")
}

func TestHandleAuthorize_WhenEmptyRedirectURI_Returns400(t *testing.T) {
	t.Parallel()

	oauth := newTestOAuth()

	req := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type": {"code"},
		"client_id":     {"test-client"},
		"state":         {"abc"},
	}.Encode(), nil)
	w := httptest.NewRecorder()

	oauth.HandleAuthorize(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, w.Header().Get("Location"))

	var errResp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_request", errResp["error"])
}

func TestHandleAuthorize_WhenValidRedirectURI_Redirects(t *testing.T) {
	t.Parallel()

	oauth := newTestOAuth()
	pkce := newTestPKCE()

	req := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {testRedirectURI},
		"state":                 {"abc"},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	w := httptest.NewRecorder()

	oauth.HandleAuthorize(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	require.NotEmpty(t, location)
	assert.True(t, strings.HasPrefix(location, testRedirectURI+"?"))
}

func TestHandleToken_WhenRedirectURIMismatch_Fails(t *testing.T) {
	t.Parallel()

	oauth := NewOAuthServer(config.AuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{testRedirectURI, "https://other.test/cb"},
	}, "https://herald.test")
	pkce := newTestPKCE()

	// Authorize with one redirect_uri
	authReq := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {testRedirectURI},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	authW := httptest.NewRecorder()
	oauth.HandleAuthorize(authW, authReq)

	redirectURL, _ := url.Parse(authW.Header().Get("Location"))
	code := redirectURL.Query().Get("code")

	// Exchange with a DIFFERENT redirect_uri
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
		"redirect_uri":  {"https://other.test/cb"},
		"code_verifier": {pkce.Verifier},
	}
	tokenReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenW := httptest.NewRecorder()
	oauth.HandleToken(tokenW, tokenReq)

	assert.Equal(t, http.StatusBadRequest, tokenW.Code)

	var errResp map[string]string
	require.NoError(t, json.Unmarshal(tokenW.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_grant", errResp["error"])
	assert.Contains(t, errResp["error_description"], "redirect_uri")
}

func TestHandleAuthorize_WhenMissingCodeChallenge_ReturnsError(t *testing.T) {
	t.Parallel()

	oauth := newTestOAuth()

	req := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type": {"code"},
		"client_id":     {"test-client"},
		"redirect_uri":  {testRedirectURI},
		"state":         {"abc"},
	}.Encode(), nil)
	w := httptest.NewRecorder()

	oauth.HandleAuthorize(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	require.NotEmpty(t, location)

	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	assert.Equal(t, "invalid_request", redirectURL.Query().Get("error"))
	assert.Contains(t, redirectURL.Query().Get("error_description"), "code_challenge")
}

func TestHandleAuthorize_WhenWrongChallengeMethod_ReturnsError(t *testing.T) {
	t.Parallel()

	oauth := newTestOAuth()
	pkce := newTestPKCE()

	req := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {testRedirectURI},
		"state":                 {"abc"},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"plain"},
	}.Encode(), nil)
	w := httptest.NewRecorder()

	oauth.HandleAuthorize(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	require.NotEmpty(t, location)

	redirectURL, err := url.Parse(location)
	require.NoError(t, err)
	assert.Equal(t, "invalid_request", redirectURL.Query().Get("error"))
	assert.Contains(t, redirectURL.Query().Get("error_description"), "S256")
}

func TestHandleToken_WhenMissingCodeVerifier_ReturnsError(t *testing.T) {
	t.Parallel()

	oauth := newTestOAuth()
	pkce := newTestPKCE()

	// Authorize with PKCE
	authReq := httptest.NewRequest("GET", "/oauth/authorize?"+url.Values{
		"response_type":         {"code"},
		"client_id":             {"test-client"},
		"redirect_uri":          {testRedirectURI},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
	}.Encode(), nil)
	authW := httptest.NewRecorder()
	oauth.HandleAuthorize(authW, authReq)

	redirectURL, _ := url.Parse(authW.Header().Get("Location"))
	code := redirectURL.Query().Get("code")
	require.NotEmpty(t, code)

	// Exchange WITHOUT code_verifier
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
		"redirect_uri":  {testRedirectURI},
	}
	tokenReq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenW := httptest.NewRecorder()
	oauth.HandleToken(tokenW, tokenReq)

	assert.Equal(t, http.StatusBadRequest, tokenW.Code)

	var errResp map[string]string
	require.NoError(t, json.Unmarshal(tokenW.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_grant", errResp["error"])
	assert.Contains(t, errResp["error_description"], "code_verifier")
}

// computeS256Challenge generates a PKCE S256 challenge from a verifier.
func computeS256Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
