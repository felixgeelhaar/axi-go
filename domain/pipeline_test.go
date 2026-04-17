package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/felixgeelhaar/axi-go/domain"
)

// recordingInvoker is a minimal CapabilityInvoker backed by a map of fakes.
// Unit tests of Pipeline don't need the full execution service wired up.
type recordingInvoker struct {
	executors map[domain.CapabilityName]func(any) (any, error)
	calls     []domain.CapabilityName
}

func (r *recordingInvoker) Invoke(name domain.CapabilityName, input any) (any, error) {
	r.calls = append(r.calls, name)
	fn, ok := r.executors[name]
	if !ok {
		return nil, errors.New("no executor for " + string(name))
	}
	return fn(input)
}

func TestPipeline_Success(t *testing.T) {
	inv := &recordingInvoker{executors: map[domain.CapabilityName]func(any) (any, error){
		"double":  func(v any) (any, error) { return v.(int) * 2, nil },
		"add-one": func(v any) (any, error) { return v.(int) + 1, nil },
	}}
	p := domain.NewPipeline("double", "add-one")

	out, err := p.ExecuteWithInvoker(context.Background(), 5, inv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != 11 {
		t.Errorf("expected 11, got %v", out)
	}
}

func TestPipeline_FailureCarriesCompletedOutputs(t *testing.T) {
	inv := &recordingInvoker{executors: map[domain.CapabilityName]func(any) (any, error){
		"double":  func(v any) (any, error) { return v.(int) * 2, nil },
		"add-one": func(v any) (any, error) { return v.(int) + 1, nil },
		"boom":    func(any) (any, error) { return nil, errors.New("nope") },
	}}
	p := domain.NewPipeline("double", "add-one", "boom", "add-one")

	_, err := p.ExecuteWithInvoker(context.Background(), 5, inv)
	if err == nil {
		t.Fatal("expected error")
	}
	var pf *domain.PipelineFailure
	if !errors.As(err, &pf) {
		t.Fatalf("expected *PipelineFailure, got %T: %v", err, err)
	}
	if pf.FailedStep != 2 {
		t.Errorf("FailedStep = %d, want 2", pf.FailedStep)
	}
	if len(pf.CompletedOutput) != 2 {
		t.Fatalf("CompletedOutput len = %d, want 2", len(pf.CompletedOutput))
	}
	if pf.CompletedOutput[0] != 10 { // 5*2
		t.Errorf("CompletedOutput[0] = %v, want 10", pf.CompletedOutput[0])
	}
	if pf.CompletedOutput[1] != 11 { // 10+1
		t.Errorf("CompletedOutput[1] = %v, want 11", pf.CompletedOutput[1])
	}
	if pf.Cause == nil || pf.Cause.Error() != "nope" {
		t.Errorf("Cause = %v, want 'nope'", pf.Cause)
	}
	// Short-circuits: step 4 ("add-one" again) must not be invoked.
	if len(inv.calls) != 3 {
		t.Errorf("expected 3 invocations before short-circuit, got %d (%v)", len(inv.calls), inv.calls)
	}
}

func TestPipeline_FailureOnFirstStep(t *testing.T) {
	inv := &recordingInvoker{executors: map[domain.CapabilityName]func(any) (any, error){
		"boom": func(any) (any, error) { return nil, errors.New("first") },
	}}
	p := domain.NewPipeline("boom")

	_, err := p.ExecuteWithInvoker(context.Background(), 1, inv)
	var pf *domain.PipelineFailure
	if !errors.As(err, &pf) {
		t.Fatalf("expected *PipelineFailure, got %T", err)
	}
	if pf.FailedStep != 0 {
		t.Errorf("FailedStep = %d, want 0", pf.FailedStep)
	}
	if len(pf.CompletedOutput) != 0 {
		t.Errorf("CompletedOutput should be empty, got %v", pf.CompletedOutput)
	}
}

func TestPipeline_FailureErrorMessage(t *testing.T) {
	pf := &domain.PipelineFailure{
		FailedStep:      3,
		CompletedOutput: []any{"a", "b", "c"},
		Cause:           errors.New("upstream 500"),
	}
	msg := pf.Error()
	for _, want := range []string{"step 3", "3 completed", "upstream 500"} {
		if !contains(msg, want) {
			t.Errorf("error message missing %q: %s", want, msg)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
