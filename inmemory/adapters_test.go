package inmemory_test

import (
	"context"
	"testing"

	"go.klarlabs.de/axi/domain"
	"go.klarlabs.de/axi/inmemory"
)

func TestActionDefinitionRepository_CRUD(t *testing.T) {
	repo := inmemory.NewActionDefinitionRepository()
	action, err := domain.NewActionDefinition("greet", "g",
		domain.EmptyContract(), domain.EmptyContract(), nil,
		domain.EffectProfile{}, domain.IdempotencyProfile{})
	if err != nil {
		t.Fatalf("NewActionDefinition: %v", err)
	}

	if _, err := repo.GetByName("greet"); err == nil {
		t.Fatal("expected not found before Save")
	}
	if err := repo.Save(action); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.GetByName("greet")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.Name() != "greet" {
		t.Errorf("Name = %s", got.Name())
	}
	if len(repo.List()) != 1 {
		t.Errorf("List len = %d", len(repo.List()))
	}
	if err := repo.Delete("greet"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByName("greet"); err == nil {
		t.Fatal("expected not found after Delete")
	}
}

func TestCapabilityDefinitionRepository_CRUD(t *testing.T) {
	repo := inmemory.NewCapabilityDefinitionRepository()
	cap, _ := domain.NewCapabilityDefinition("string.upper", "u", domain.EmptyContract(), domain.EmptyContract())
	if err := repo.Save(cap); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.GetByName("string.upper")
	if err != nil || got.Name() != "string.upper" {
		t.Fatalf("GetByName: %v %#v", err, got)
	}
	if len(repo.List()) != 1 {
		t.Errorf("List len = %d", len(repo.List()))
	}
	_ = repo.Delete("string.upper")
	if _, err := repo.GetByName("string.upper"); err == nil {
		t.Fatal("expected not found")
	}
}

func TestPluginContributionRepository_CRUD(t *testing.T) {
	repo := inmemory.NewPluginContributionRepository()
	contrib, _ := domain.NewPluginContribution("p1", nil, nil)
	if repo.Exists("p1") {
		t.Fatal("Exists before Save")
	}
	if err := repo.Save(contrib); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !repo.Exists("p1") {
		t.Fatal("Exists after Save")
	}
	got, err := repo.GetByID("p1")
	if err != nil || got.PluginID() != "p1" {
		t.Fatalf("GetByID: %v %#v", err, got)
	}
	if err := repo.Delete("p1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.Exists("p1") {
		t.Fatal("Exists after Delete")
	}
}

func TestExecutionSessionRepository_SaveGet(t *testing.T) {
	repo := inmemory.NewExecutionSessionRepository()
	session, _ := domain.NewExecutionSession("s1", "greet", nil)
	if err := repo.Save(session); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Get("s1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID() != "s1" {
		t.Errorf("ID = %s", got.ID())
	}
	if _, err := repo.Get("missing"); err == nil {
		t.Fatal("expected not found")
	}
}

type noopActionExec struct{}

func (noopActionExec) Execute(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	return domain.ExecutionResult{Summary: "ok"}, nil, nil
}

type noopCapExec struct{}

func (noopCapExec) Execute(_ context.Context, _ any) (any, error) { return "ok", nil }

func TestActionExecutorRegistry_RegisterGetUnregister(t *testing.T) {
	reg := inmemory.NewActionExecutorRegistry()
	exec := noopActionExec{}
	reg.Register("exec.a", exec)
	got, err := reg.GetActionExecutor("exec.a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, ok := got.(noopActionExec); !ok {
		t.Fatalf("got type %T", got)
	}
	reg.Unregister("exec.a")
	if _, err := reg.GetActionExecutor("exec.a"); err == nil {
		t.Fatal("expected missing after Unregister")
	}
}

func TestCapabilityExecutorRegistry_RegisterGetUnregister(t *testing.T) {
	reg := inmemory.NewCapabilityExecutorRegistry()
	exec := noopCapExec{}
	reg.Register("exec.c", exec)
	got, err := reg.GetCapabilityExecutor("exec.c")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, ok := got.(noopCapExec); !ok {
		t.Fatalf("got type %T", got)
	}
	reg.Unregister("exec.c")
	if _, err := reg.GetCapabilityExecutor("exec.c"); err == nil {
		t.Fatal("expected missing after Unregister")
	}
}

func TestSequentialIDGenerator(t *testing.T) {
	gen := inmemory.NewSequentialIDGenerator()
	a := gen.GenerateSessionID()
	b := gen.GenerateSessionID()
	if a != "session-1" || b != "session-2" {
		t.Errorf("got %s, %s", a, b)
	}
}

func TestStdLogger_DoesNotPanic(t *testing.T) {
	l := inmemory.NewStdLogger(inmemory.LevelDebug)
	l.Debug("d", domain.F("k", 1))
	l.Info("i")
	l.Warn("w", domain.F("x", "y"))
	l.Error("e")

	quiet := inmemory.NewStdLogger(inmemory.LevelError)
	quiet.Debug("suppressed")
	quiet.Info("suppressed")
	quiet.Warn("suppressed")
	quiet.Error("emitted", domain.F("ok", true))
}
