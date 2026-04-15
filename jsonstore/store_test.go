package jsonstore_test

import (
	"testing"

	"github.com/felixgeelhaar/axi-go/domain"
	"github.com/felixgeelhaar/axi-go/jsonstore"
)

func TestActionStore_SaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := jsonstore.NewActionStore(dir)
	if err != nil {
		t.Fatalf("NewActionStore: %v", err)
	}

	action, _ := domain.NewActionDefinition("greet", "Greet someone",
		domain.NewContract([]domain.ContractField{
			{Name: "name", Type: "string", Description: "Person to greet", Required: true},
		}),
		domain.EmptyContract(), nil,
		domain.EffectProfile{Level: domain.EffectNone},
		domain.IdempotencyProfile{IsIdempotent: true},
	)
	_ = action.BindExecutor("exec.greet")

	if err := store.Save(action); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.GetByName("greet")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.Name() != "greet" {
		t.Errorf("expected greet, got %s", got.Name())
	}
	if got.Description() != "Greet someone" {
		t.Errorf("expected description, got %s", got.Description())
	}
	if !got.IsBound() {
		t.Error("expected bound")
	}
	if got.InputContract().Fields[0].Type != "string" {
		t.Error("expected string type on input field")
	}
}

func TestActionStore_List(t *testing.T) {
	dir := t.TempDir()
	store, _ := jsonstore.NewActionStore(dir)

	a1, _ := domain.NewActionDefinition("a", "A", domain.EmptyContract(), domain.EmptyContract(), nil, domain.EffectProfile{}, domain.IdempotencyProfile{})
	_ = a1.BindExecutor("e1")
	a2, _ := domain.NewActionDefinition("b", "B", domain.EmptyContract(), domain.EmptyContract(), nil, domain.EffectProfile{}, domain.IdempotencyProfile{})
	_ = a2.BindExecutor("e2")

	_ = store.Save(a1)
	_ = store.Save(a2)

	list := store.List()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestActionStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, _ := jsonstore.NewActionStore(dir)

	a, _ := domain.NewActionDefinition("temp", "Temp", domain.EmptyContract(), domain.EmptyContract(), nil, domain.EffectProfile{}, domain.IdempotencyProfile{})
	_ = a.BindExecutor("e")
	_ = store.Save(a)

	if err := store.Delete("temp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.GetByName("temp"); err == nil {
		t.Error("expected not found after delete")
	}
}

func TestActionStore_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := jsonstore.NewActionStore(dir)

	_, err := store.GetByName("missing")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCapabilityStore_SaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store, _ := jsonstore.NewCapabilityStore(dir)

	cap, _ := domain.NewCapabilityDefinition("http.get", "HTTP GET",
		domain.NewContract([]domain.ContractField{{Name: "url", Type: "string", Required: true}}),
		domain.EmptyContract(),
	)
	_ = cap.BindExecutor("exec.http")

	_ = store.Save(cap)
	got, err := store.GetByName("http.get")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.Name() != "http.get" {
		t.Errorf("expected http.get, got %s", got.Name())
	}
}

func TestPluginStore_SaveExistsDelete(t *testing.T) {
	dir := t.TempDir()
	store, _ := jsonstore.NewPluginStore(dir)

	a, _ := domain.NewActionDefinition("act", "A", domain.EmptyContract(), domain.EmptyContract(), nil, domain.EffectProfile{}, domain.IdempotencyProfile{})
	_ = a.BindExecutor("e")
	p, _ := domain.NewPluginContribution("test.plugin", []*domain.ActionDefinition{a}, nil)
	_ = p.Activate()

	_ = store.Save(p)

	if !store.Exists("test.plugin") {
		t.Error("expected exists")
	}

	_, err := store.GetByID("test.plugin")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	_ = store.Delete("test.plugin")
	if store.Exists("test.plugin") {
		t.Error("expected not exists after delete")
	}
}

func TestSessionStore_SaveAndGet_Pending(t *testing.T) {
	dir := t.TempDir()
	store, _ := jsonstore.NewSessionStore(dir)

	session, _ := domain.NewExecutionSession("s1", "greet", map[string]any{"name": "world"})

	if err := store.Save(session); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Get("s1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID() != "s1" {
		t.Errorf("expected s1, got %s", got.ID())
	}
	if got.ActionName() != "greet" {
		t.Errorf("expected greet, got %s", got.ActionName())
	}
	if got.Status() != domain.StatusPending {
		t.Errorf("expected pending, got %s", got.Status())
	}
}

func TestSessionStore_SaveAndGet_Succeeded(t *testing.T) {
	dir := t.TempDir()
	store, _ := jsonstore.NewSessionStore(dir)

	session, _ := domain.NewExecutionSession("s2", "greet", map[string]any{"name": "world"})
	_ = session.MarkValidated()
	_ = session.MarkResolved([]domain.CapabilityName{"string.upper"})
	_ = session.MarkRunning()
	session.AppendEvidence(domain.EvidenceRecord{Kind: "log", Source: "test", Value: "ran", Timestamp: 1234567890})
	_ = session.Succeed(domain.ExecutionResult{Data: "Hello!", Summary: "greeted", ContentType: "text/plain"})

	_ = store.Save(session)
	got, err := store.Get("s2")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status() != domain.StatusSucceeded {
		t.Errorf("expected succeeded, got %s", got.Status())
	}
	if got.Result() == nil || got.Result().Data != "Hello!" {
		t.Errorf("expected result Hello!, got %v", got.Result())
	}
	if got.Result().ContentType != "text/plain" {
		t.Errorf("expected text/plain, got %s", got.Result().ContentType)
	}
	if len(got.Evidence()) != 1 || got.Evidence()[0].Timestamp != 1234567890 {
		t.Errorf("expected evidence with timestamp, got %v", got.Evidence())
	}
	if len(got.ResolvedCapabilities()) != 1 || got.ResolvedCapabilities()[0] != "string.upper" {
		t.Errorf("expected resolved capabilities, got %v", got.ResolvedCapabilities())
	}
}

func TestSessionStore_SaveAndGet_Failed(t *testing.T) {
	dir := t.TempDir()
	store, _ := jsonstore.NewSessionStore(dir)

	session, _ := domain.NewExecutionSession("s3", "fail", nil)
	_ = session.MarkValidated()
	_ = session.MarkResolved(nil)
	_ = session.MarkRunning()
	_ = session.Fail(domain.FailureReason{Code: "ERR", Message: "boom"})

	_ = store.Save(session)
	got, _ := store.Get("s3")

	if got.Status() != domain.StatusFailed {
		t.Errorf("expected failed, got %s", got.Status())
	}
	if got.Failure() == nil || got.Failure().Code != "ERR" {
		t.Errorf("expected failure reason, got %v", got.Failure())
	}
}
