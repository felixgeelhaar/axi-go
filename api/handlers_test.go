package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/axi-go/api"
	"github.com/felixgeelhaar/axi-go/application"
	"github.com/felixgeelhaar/axi-go/domain"
	"github.com/felixgeelhaar/axi-go/inmemory"
)

type stubActionExecutor struct {
	fn func(ctx context.Context, input any, caps domain.CapabilityInvokerFunc) (domain.ExecutionResult, []domain.EvidenceRecord, error)
}

func (s *stubActionExecutor) Execute(ctx context.Context, input any, caps domain.CapabilityInvokerFunc) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	return s.fn(ctx, input, caps)
}

func setupTestServer(t *testing.T) (*api.Server, *inmemory.ActionExecutorRegistry) {
	t.Helper()

	actionRepo := inmemory.NewActionDefinitionRepository()
	capRepo := inmemory.NewCapabilityDefinitionRepository()
	pluginRepo := inmemory.NewPluginContributionRepository()
	sessionRepo := inmemory.NewExecutionSessionRepository()
	validator := inmemory.NewContractValidator()
	actionExecReg := inmemory.NewActionExecutorRegistry()
	capExecReg := inmemory.NewCapabilityExecutorRegistry()
	idGen := inmemory.NewSequentialIDGenerator()

	compositionService := domain.NewCompositionService(actionRepo, capRepo, pluginRepo)
	resolutionService := domain.NewCapabilityResolutionService(capRepo)
	executionService := domain.NewActionExecutionService(
		actionRepo, resolutionService, validator, actionExecReg, capExecReg,
	)

	registerUC := &application.RegisterPluginContributionUseCase{CompositionService: compositionService}
	executeUC := &application.ExecuteActionUseCase{
		SessionRepo:      sessionRepo,
		ExecutionService: executionService,
		IDGen:            idGen,
	}

	srv := api.NewServer(executeUC, registerUC, actionRepo, capRepo, sessionRepo)
	return srv, actionExecReg
}

func doRequest(srv *api.Server, method, path string, body any) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	srv.Engine().Handler().ServeHTTP(w, req)
	return w
}

func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return result
}

// --- Tests ---

func TestListActions_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "GET", "/api/v1/actions", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var actions []api.ActionResponse
	if err := json.NewDecoder(w.Body).Decode(&actions); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected empty list, got %d", len(actions))
	}
}

func TestRegisterPlugin_AndListActions(t *testing.T) {
	srv, _ := setupTestServer(t)

	plugin := api.RegisterPluginRequest{
		PluginID: "test.plugin",
		Actions: []api.ActionDTO{
			{
				Name:        "greet",
				Description: "Greet someone",
				ExecutorRef: "exec.greet",
				InputContract: api.ContractDTO{
					Fields: []api.ContractFieldDTO{{Name: "name", Required: true}},
				},
			},
		},
	}

	w := doRequest(srv, "POST", "/api/v1/plugins", plugin)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// List should now have 1 action.
	w = doRequest(srv, "GET", "/api/v1/actions", nil)
	actions := decodeJSON[[]api.ActionResponse](t, w)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Name != "greet" {
		t.Errorf("expected greet, got %s", actions[0].Name)
	}
	if !actions[0].IsBound {
		t.Error("expected action to be bound")
	}
}

func TestGetAction_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "GET", "/api/v1/actions/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetAction_Found(t *testing.T) {
	srv, _ := setupTestServer(t)

	plugin := api.RegisterPluginRequest{
		PluginID: "p1",
		Actions:  []api.ActionDTO{{Name: "hello", Description: "Say hello", ExecutorRef: "exec.hello"}},
	}
	doRequest(srv, "POST", "/api/v1/plugins", plugin)

	w := doRequest(srv, "GET", "/api/v1/actions/hello", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	action := decodeJSON[api.ActionResponse](t, w)
	if action.Name != "hello" {
		t.Errorf("expected hello, got %s", action.Name)
	}
}

func TestExecuteAction_Success(t *testing.T) {
	srv, actionExecReg := setupTestServer(t)

	// Register a plugin with an action.
	plugin := api.RegisterPluginRequest{
		PluginID: "exec.plugin",
		Actions:  []api.ActionDTO{{Name: "echo", Description: "Echo input", ExecutorRef: "exec.echo"}},
	}
	doRequest(srv, "POST", "/api/v1/plugins", plugin)

	// Register the executor.
	actionExecReg.Register("exec.echo", &stubActionExecutor{
		fn: func(_ context.Context, input any, _ domain.CapabilityInvokerFunc) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			return domain.ExecutionResult{Data: input, Summary: "echoed"}, nil, nil
		},
	})

	// Execute.
	w := doRequest(srv, "POST", "/api/v1/actions/execute", api.ExecuteActionRequest{
		ActionName: "echo",
		Input:      map[string]any{"msg": "hi"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeJSON[api.ExecuteActionResponse](t, w)
	if resp.Status != "succeeded" {
		t.Errorf("expected succeeded, got %s", resp.Status)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
}

func TestExecuteAction_MissingActionName(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "POST", "/api/v1/actions/execute", api.ExecuteActionRequest{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestExecuteAction_ActionNotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "POST", "/api/v1/actions/execute", api.ExecuteActionRequest{
		ActionName: "nonexistent",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSession(t *testing.T) {
	srv, actionExecReg := setupTestServer(t)

	plugin := api.RegisterPluginRequest{
		PluginID: "sess.plugin",
		Actions:  []api.ActionDTO{{Name: "noop", ExecutorRef: "exec.noop"}},
	}
	doRequest(srv, "POST", "/api/v1/plugins", plugin)
	actionExecReg.Register("exec.noop", &stubActionExecutor{
		fn: func(_ context.Context, _ any, _ domain.CapabilityInvokerFunc) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			return domain.ExecutionResult{Data: "ok"}, nil, nil
		},
	})

	// Execute to create a session.
	w := doRequest(srv, "POST", "/api/v1/actions/execute", api.ExecuteActionRequest{ActionName: "noop"})
	execResp := decodeJSON[api.ExecuteActionResponse](t, w)

	// Retrieve the session.
	w = doRequest(srv, "GET", "/api/v1/sessions/"+execResp.SessionID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	sess := decodeJSON[api.SessionResponse](t, w)
	if sess.SessionID != execResp.SessionID {
		t.Errorf("session ID mismatch: %s != %s", sess.SessionID, execResp.SessionID)
	}
	if sess.Status != "succeeded" {
		t.Errorf("expected succeeded, got %s", sess.Status)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "GET", "/api/v1/sessions/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListCapabilities_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "GET", "/api/v1/capabilities", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	caps := decodeJSON[[]api.CapabilityResponse](t, w)
	if len(caps) != 0 {
		t.Errorf("expected empty, got %d", len(caps))
	}
}

func TestRegisterPlugin_WithCapabilities(t *testing.T) {
	srv, _ := setupTestServer(t)

	plugin := api.RegisterPluginRequest{
		PluginID: "cap.plugin",
		Capabilities: []api.CapabilityDTO{
			{Name: "http.get", Description: "HTTP GET", ExecutorRef: "exec.http.get"},
		},
	}
	w := doRequest(srv, "POST", "/api/v1/plugins", plugin)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(srv, "GET", "/api/v1/capabilities", nil)
	caps := decodeJSON[[]api.CapabilityResponse](t, w)
	if len(caps) != 1 {
		t.Fatalf("expected 1, got %d", len(caps))
	}
	if caps[0].Name != "http.get" {
		t.Errorf("expected http.get, got %s", caps[0].Name)
	}
}

func TestRegisterPlugin_DuplicateRejected(t *testing.T) {
	srv, _ := setupTestServer(t)

	plugin := api.RegisterPluginRequest{PluginID: "dup"}
	doRequest(srv, "POST", "/api/v1/plugins", plugin)

	w := doRequest(srv, "POST", "/api/v1/plugins", plugin)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterPlugin_BadRequest(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "POST", "/api/v1/plugins", api.RegisterPluginRequest{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
