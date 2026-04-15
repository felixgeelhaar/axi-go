package api

import "net/http"

// Authenticator is the interface for request authentication.
// Implementations can verify tokens, API keys, JWTs, etc.
type Authenticator interface {
	// Authenticate checks the request and returns an error if unauthorized.
	Authenticate(r *http.Request) error
}

// AuthMiddleware creates a middleware that rejects unauthenticated requests.
// Paths in skipPaths are excluded from auth (e.g., "/health").
func AuthMiddleware(auth Authenticator, skipPaths ...string) HandlerFunc {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}
	return func(c *Context) {
		if _, ok := skip[c.Request.URL.Path]; ok {
			return
		}
		if err := auth.Authenticate(c.Request); err != nil {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:     err.Error(),
				ErrorCode: "unauthorized",
			})
			return
		}
	}
}

// BearerTokenAuth is a simple token-based authenticator.
// It checks the Authorization header for "Bearer <token>".
type BearerTokenAuth struct {
	validTokens map[string]struct{}
}

// NewBearerTokenAuth creates an authenticator that accepts the given tokens.
func NewBearerTokenAuth(tokens ...string) *BearerTokenAuth {
	m := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		m[t] = struct{}{}
	}
	return &BearerTokenAuth{validTokens: m}
}

// Authenticate checks for a valid Bearer token.
func (a *BearerTokenAuth) Authenticate(r *http.Request) error {
	header := r.Header.Get("Authorization")
	if len(header) < 8 || header[:7] != "Bearer " {
		return &authError{msg: "missing or invalid Authorization header"}
	}
	token := header[7:]
	if _, ok := a.validTokens[token]; !ok {
		return &authError{msg: "invalid token"}
	}
	return nil
}

type authError struct {
	msg string
}

func (e *authError) Error() string {
	return e.msg
}
