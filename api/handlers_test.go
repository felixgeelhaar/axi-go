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
	fn func(ctx context.Context, input any, caps domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error)
}

func (s *stubActionExecutor) Execute(ctx context.Context, input any, caps domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
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
		t.Fatalf("failed to decode response: %v\nbody: %s", err, w.Body.String())
	}
	return result
}

// --- Tests ---

func TestHealthCheck(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "GET", "/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeJSON[api.HealthResponse](t, w)
	if resp.Status != "ok" {
		t.Errorf("expected ok, got %s", resp.Status)
	}
}

func TestListActions_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "GET", "/api/v1/actions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeJSON[api.ListResponse[api.ActionResponse]](t, w)
	if resp.Count != 0 {
		t.Errorf("expected 0, got %d", resp.Count)
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
					Fields: []api.ContractFieldDTO{
						{Name: "name", Type: "string", Description: "Person to greet", Required: true, Example: "world"},
					},
				},
			},
		},
	}

	w := doRequest(srv, "POST", "/api/v1/plugins", plugin)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	regResp := decodeJSON[api.RegisterPluginResponse](t, w)
	if regResp.ActionCount != 1 {
		t.Errorf("expected 1 action, got %d", regResp.ActionCount)
	}

	w = doRequest(srv, "GET", "/api/v1/actions", nil)
	listResp := decodeJSON[api.ListResponse[api.ActionResponse]](t, w)
	if listResp.Count != 1 {
		t.Fatalf("expected 1 action, got %d", listResp.Count)
	}
	if listResp.Items[0].Name != "greet" {
		t.Errorf("expected greet, got %s", listResp.Items[0].Name)
	}
	if listResp.Items[0].InputContract.Fields[0].Type != "string" {
		t.Errorf("expected string type, got %s", listResp.Items[0].InputContract.Fields[0].Type)
	}
}

func TestGetAction_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "GET", "/api/v1/actions/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	resp := decodeJSON[api.ErrorResponse](t, w)
	if resp.ErrorCode != "not_found" {
		t.Errorf("expected not_found error code, got %s", resp.ErrorCode)
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

	plugin := api.RegisterPluginRequest{
		PluginID: "exec.plugin",
		Actions:  []api.ActionDTO{{Name: "echo", Description: "Echo input", ExecutorRef: "exec.echo"}},
	}
	doRequest(srv, "POST", "/api/v1/plugins", plugin)

	actionExecReg.Register("exec.echo", &stubActionExecutor{
		fn: func(_ context.Context, input any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			return domain.ExecutionResult{Data: input, Summary: "echoed", ContentType: "application/json"}, nil, nil
		},
	})

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
	if resp.Result.ContentType != "application/json" {
		t.Errorf("expected application/json content type, got %s", resp.Result.ContentType)
	}
}

func TestExecuteAction_MissingActionName(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "POST", "/api/v1/actions/execute", api.ExecuteActionRequest{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	resp := decodeJSON[api.ErrorResponse](t, w)
	if resp.ErrorCode != "validation_error" {
		t.Errorf("expected validation_error, got %s", resp.ErrorCode)
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
	resp := decodeJSON[api.ErrorResponse](t, w)
	if resp.ErrorCode != "not_found" {
		t.Errorf("expected not_found, got %s", resp.ErrorCode)
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
		fn: func(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			return domain.ExecutionResult{Data: "ok"}, nil, nil
		},
	})

	w := doRequest(srv, "POST", "/api/v1/actions/execute", api.ExecuteActionRequest{ActionName: "noop"})
	execResp := decodeJSON[api.ExecuteActionResponse](t, w)

	w = doRequest(srv, "GET", "/api/v1/sessions/"+execResp.SessionID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	sess := decodeJSON[api.SessionResponse](t, w)
	if sess.SessionID != execResp.SessionID {
		t.Errorf("session ID mismatch")
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
	resp := decodeJSON[api.ListResponse[api.CapabilityResponse]](t, w)
	if resp.Count != 0 {
		t.Errorf("expected 0, got %d", resp.Count)
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
	resp := decodeJSON[api.ListResponse[api.CapabilityResponse]](t, w)
	if resp.Count != 1 {
		t.Fatalf("expected 1, got %d", resp.Count)
	}
	if resp.Items[0].Name != "http.get" {
		t.Errorf("expected http.get, got %s", resp.Items[0].Name)
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
	resp := decodeJSON[api.ErrorResponse](t, w)
	if resp.ErrorCode != "conflict" {
		t.Errorf("expected conflict, got %s", resp.ErrorCode)
	}
}

func TestRegisterPlugin_BadRequest(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(srv, "POST", "/api/v1/plugins", api.RegisterPluginRequest{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Approval flow tests ---

func TestExecuteAction_ExternalEffect_PausesForApproval(t *testing.T) {
	srv, actionExecReg := setupTestServer(t)

	// Register a plugin with an external-effect action.
	plugin := api.RegisterPluginRequest{
		PluginID: "ext.plugin",
		Actions: []api.ActionDTO{{
			Name:        "send-email",
			Description: "Sends an email (external effect)",
			ExecutorRef: "exec.email",
			EffectLevel: "external",
		}},
	}
	doRequest(srv, "POST", "/api/v1/plugins", plugin)

	actionExecReg.Register("exec.email", &stubActionExecutor{
		fn: func(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			return domain.ExecutionResult{Data: "sent", ContentType: "text/plain"}, nil, nil
		},
	})

	// Execute — should pause at awaiting_approval.
	w := doRequest(srv, "POST", "/api/v1/actions/execute", api.ExecuteActionRequest{ActionName: "send-email"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeJSON[api.ExecuteActionResponse](t, w)
	if resp.Status != "awaiting_approval" {
		t.Fatalf("expected awaiting_approval, got %s", resp.Status)
	}
	if !resp.RequiresApproval {
		t.Error("expected requires_approval to be true")
	}

	// Approve — should complete execution.
	w = doRequest(srv, "POST", "/api/v1/sessions/"+resp.SessionID+"/approve", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	approved := decodeJSON[api.ExecuteActionResponse](t, w)
	if approved.Status != "succeeded" {
		t.Errorf("expected succeeded, got %s", approved.Status)
	}
	if approved.Result == nil || approved.Result.Data != "sent" {
		t.Error("expected result data 'sent'")
	}
}

func TestExecuteAction_ExternalEffect_Rejected(t *testing.T) {
	srv, actionExecReg := setupTestServer(t)

	plugin := api.RegisterPluginRequest{
		PluginID: "rej.plugin",
		Actions: []api.ActionDTO{{
			Name:        "delete-account",
			Description: "Deletes an account (external effect)",
			ExecutorRef: "exec.delete",
			EffectLevel: "external",
		}},
	}
	doRequest(srv, "POST", "/api/v1/plugins", plugin)

	actionExecReg.Register("exec.delete", &stubActionExecutor{
		fn: func(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			return domain.ExecutionResult{Data: "deleted"}, nil, nil
		},
	})

	// Execute — pauses.
	w := doRequest(srv, "POST", "/api/v1/actions/execute", api.ExecuteActionRequest{ActionName: "delete-account"})
	resp := decodeJSON[api.ExecuteActionResponse](t, w)
	if resp.Status != "awaiting_approval" {
		t.Fatalf("expected awaiting_approval, got %s", resp.Status)
	}

	// Reject.
	w = doRequest(srv, "POST", "/api/v1/sessions/"+resp.SessionID+"/reject", api.RejectSessionRequest{Reason: "too dangerous"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	rejected := decodeJSON[api.ExecuteActionResponse](t, w)
	if rejected.Status != "rejected" {
		t.Errorf("expected rejected, got %s", rejected.Status)
	}
	if rejected.Failure == nil || rejected.Failure.Code != "REJECTED" {
		t.Error("expected REJECTED failure code")
	}
}

func TestApproveSession_NotAwaitingApproval(t *testing.T) {
	srv, actionExecReg := setupTestServer(t)

	plugin := api.RegisterPluginRequest{
		PluginID: "safe.plugin",
		Actions:  []api.ActionDTO{{Name: "safe-action", ExecutorRef: "exec.safe"}},
	}
	doRequest(srv, "POST", "/api/v1/plugins", plugin)
	actionExecReg.Register("exec.safe", &stubActionExecutor{
		fn: func(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			return domain.ExecutionResult{Data: "ok"}, nil, nil
		},
	})

	// Execute — completes immediately (no approval needed).
	w := doRequest(srv, "POST", "/api/v1/actions/execute", api.ExecuteActionRequest{ActionName: "safe-action"})
	resp := decodeJSON[api.ExecuteActionResponse](t, w)
	if resp.Status != "succeeded" {
		t.Fatalf("expected succeeded, got %s", resp.Status)
	}

	// Try to approve a succeeded session — should fail.
	w = doRequest(srv, "POST", "/api/v1/sessions/"+resp.SessionID+"/approve", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
