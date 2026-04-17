package main

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"

	"github.com/felixgeelhaar/axi-go"
)

// TestServe_InitializeListCall drives the full dispatch chain through stdio
// semantics. Protects the example from silent rot as axi-go evolves.
func TestServe_InitializeListCall(t *testing.T) {
	kernel := axi.New().WithBudget(axi.Budget{MaxCapabilityInvocations: 10})
	kernel.RegisterActionExecutor("exec.echo.upper", &upperExecutor{})
	if err := kernel.RegisterPlugin(&echoPlugin{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	in := strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo.upper","arguments":{"text":"hello"}}}`,
	}, "\n") + "\n")

	var out bytes.Buffer
	serve(in, &out, log.New(bytes.NewBuffer(nil), "", 0), kernel)

	decoder := json.NewDecoder(&out)

	// 1. initialize
	var init rpcResponse
	if err := decoder.Decode(&init); err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	if init.Error != nil {
		t.Fatalf("initialize error: %+v", init.Error)
	}

	// 2. tools/list — must include echo.upper
	var list rpcResponse
	if err := decoder.Decode(&list); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	listJSON, _ := json.Marshal(list.Result)
	if !strings.Contains(string(listJSON), `"echo.upper"`) {
		t.Errorf("tools/list missing echo.upper; got: %s", listJSON)
	}
	if !strings.Contains(string(listJSON), `"required":["text"]`) {
		t.Errorf("tools/list schema missing required field; got: %s", listJSON)
	}

	// 3. tools/call — expect TOON-encoded uppercase result in content[0].text
	var call rpcResponse
	if err := decoder.Decode(&call); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	if call.Error != nil {
		t.Fatalf("tools/call error: %+v", call.Error)
	}
	callJSON, _ := json.Marshal(call.Result)
	if !strings.Contains(string(callJSON), "HELLO") {
		t.Errorf("tools/call result missing HELLO; got: %s", callJSON)
	}
}

func TestServe_UnknownMethod(t *testing.T) {
	kernel := axi.New()

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"does.not.exist"}` + "\n")
	var out bytes.Buffer
	serve(in, &out, log.New(bytes.NewBuffer(nil), "", 0), kernel)

	var resp rpcResponse
	if err := json.NewDecoder(&out).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if !strings.Contains(resp.Error.Message, "does.not.exist") {
		t.Errorf("error should mention the method: %s", resp.Error.Message)
	}
}
