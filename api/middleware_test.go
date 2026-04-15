package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/axi-go/api"
)

func TestBearerTokenAuth_ValidToken(t *testing.T) {
	auth := api.NewBearerTokenAuth("secret-token")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret-token")

	if err := auth.Authenticate(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBearerTokenAuth_InvalidToken(t *testing.T) {
	auth := api.NewBearerTokenAuth("secret-token")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	if err := auth.Authenticate(req); err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestBearerTokenAuth_MissingHeader(t *testing.T) {
	auth := api.NewBearerTokenAuth("secret-token")
	req := httptest.NewRequest("GET", "/", nil)

	if err := auth.Authenticate(req); err == nil {
		t.Error("expected error for missing header")
	}
}

func TestBearerTokenAuth_MalformedHeader(t *testing.T) {
	auth := api.NewBearerTokenAuth("secret-token")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic abc123")

	if err := auth.Authenticate(req); err == nil {
		t.Error("expected error for non-Bearer auth")
	}
}

func TestAuthMiddleware_BlocksUnauthenticated(t *testing.T) {
	engine := api.New()
	auth := api.NewBearerTokenAuth("my-token")
	engine.Use(api.AuthMiddleware(auth, "/health"))
	engine.GET("/api/v1/actions", func(c *api.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/api/v1/actions", nil)
	w := httptest.NewRecorder()
	engine.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_AllowsAuthenticated(t *testing.T) {
	engine := api.New()
	auth := api.NewBearerTokenAuth("my-token")
	engine.Use(api.AuthMiddleware(auth, "/health"))
	engine.GET("/api/v1/actions", func(c *api.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/api/v1/actions", nil)
	req.Header.Set("Authorization", "Bearer my-token")
	w := httptest.NewRecorder()
	engine.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_SkipsHealth(t *testing.T) {
	engine := api.New()
	auth := api.NewBearerTokenAuth("my-token")
	engine.Use(api.AuthMiddleware(auth, "/health"))
	engine.GET("/health", func(c *api.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	engine.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (skipped auth), got %d", w.Code)
	}
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	cfg := api.DefaultConfig()
	if cfg.Addr != ":8080" {
		t.Errorf("expected :8080, got %s", cfg.Addr)
	}
	if cfg.DefaultBudget.MaxCapabilityInvocations != 100 {
		t.Errorf("expected 100, got %d", cfg.DefaultBudget.MaxCapabilityInvocations)
	}
}

func TestConfigFromEnv_ReadsEnv(t *testing.T) {
	t.Setenv("AXI_ADDR", ":9090")
	t.Setenv("AXI_MAX_INVOCATIONS", "50")

	cfg := api.ConfigFromEnv()
	if cfg.Addr != ":9090" {
		t.Errorf("expected :9090, got %s", cfg.Addr)
	}
	if cfg.DefaultBudget.MaxCapabilityInvocations != 50 {
		t.Errorf("expected 50, got %d", cfg.DefaultBudget.MaxCapabilityInvocations)
	}
}
