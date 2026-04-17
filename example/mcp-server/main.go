// Command mcp-server exposes an axi-go kernel as a Model Context Protocol
// (MCP) tool provider over stdio. It's ~250 lines of Go with no external
// dependencies — a concrete demonstration that an axi-go kernel is the
// natural execution layer under any agent-facing protocol.
//
// Wire protocol: JSON-RPC 2.0 over newline-delimited stdin/stdout, following
// the MCP spec (initialize, tools/list, tools/call). Logs go to stderr.
//
// Run from another process that speaks MCP, or exercise manually:
//
//	echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | go run ./example/mcp-server
//	echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | go run ./example/mcp-server
//	echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo.upper","arguments":{"text":"hello"}}}' | go run ./example/mcp-server
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/felixgeelhaar/axi-go"
	"github.com/felixgeelhaar/axi-go/domain"
	"github.com/felixgeelhaar/axi-go/toon"
)

// --- JSON-RPC 2.0 envelopes ---

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- MCP message shapes (subset) ---

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      map[string]any `json:"serverInfo"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type listToolsResult struct {
	Tools []tool `json:"tools"`
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type callToolResult struct {
	Content []content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// --- Demo plugin ---

type echoPlugin struct{}

func (p *echoPlugin) Contribute() (*domain.PluginContribution, error) {
	action, _ := domain.NewActionDefinition(
		"echo.upper",
		"Uppercases the provided text",
		domain.NewContract([]domain.ContractField{
			{Name: "text", Type: "string", Description: "Text to uppercase", Required: true, Example: "hello"},
		}),
		domain.NewContract([]domain.ContractField{
			{Name: "result", Type: "string", Description: "The uppercased text", Required: true},
		}),
		nil,
		domain.EffectProfile{Level: domain.EffectNone},
		domain.IdempotencyProfile{IsIdempotent: true},
	)
	_ = action.BindExecutor("exec.echo.upper")
	return domain.NewPluginContribution("echo.plugin",
		[]*domain.ActionDefinition{action}, nil)
}

type upperExecutor struct{}

func (e *upperExecutor) Execute(_ context.Context, input any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	args, _ := input.(map[string]any)
	text, _ := args["text"].(string)
	return domain.ExecutionResult{
		Data:    map[string]any{"result": strings.ToUpper(text)},
		Summary: "uppercased " + text,
	}, nil, nil
}

// --- Server ---

func main() {
	logger := log.New(os.Stderr, "[mcp] ", log.LstdFlags)

	kernel := axi.New().WithBudget(axi.Budget{MaxCapabilityInvocations: 10})
	kernel.RegisterActionExecutor("exec.echo.upper", &upperExecutor{})
	if err := kernel.RegisterPlugin(&echoPlugin{}); err != nil {
		logger.Fatalf("register: %v", err)
	}

	logger.Printf("axi-go MCP server ready; %d tool(s) registered", kernel.ListActionsResult().TotalCount)
	serve(os.Stdin, os.Stdout, logger, kernel)
}

func serve(r io.Reader, w io.Writer, logger *log.Logger, kernel *axi.Kernel) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	enc := json.NewEncoder(w)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			logger.Printf("parse error: %v", err)
			continue
		}

		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		result, err := dispatch(kernel, req)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
		} else {
			resp.Result = result
		}
		if err := enc.Encode(resp); err != nil {
			logger.Printf("write error: %v", err)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Printf("read error: %v", err)
	}
}

func dispatch(kernel *axi.Kernel, req rpcRequest) (any, error) {
	switch req.Method {
	case "initialize":
		return initializeResult{
			ProtocolVersion: "2025-06-18",
			Capabilities:    map[string]any{"tools": map[string]any{}},
			ServerInfo: map[string]any{
				"name":    "axi-go-mcp-example",
				"version": "0.1.0",
			},
		}, nil

	case "tools/list":
		return handleList(kernel), nil

	case "tools/call":
		var params callToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		return handleCall(kernel, params), nil

	default:
		return nil, fmt.Errorf("unknown method: %s", req.Method)
	}
}

func handleList(kernel *axi.Kernel) listToolsResult {
	actions := kernel.ListActionsResult()
	tools := make([]tool, 0, actions.TotalCount)
	for _, a := range actions.Items {
		tools = append(tools, tool{
			Name:        string(a.Name()),
			Description: a.Description(),
			InputSchema: contractToJSONSchema(a.InputContract()),
		})
	}
	return listToolsResult{Tools: tools}
}

func handleCall(kernel *axi.Kernel, params callToolParams) callToolResult {
	result, err := kernel.Execute(context.Background(), axi.Invocation{
		Action: params.Name,
		Input:  params.Arguments,
	})
	if err != nil {
		return callToolResult{IsError: true, Content: []content{{Type: "text", Text: err.Error()}}}
	}

	// Token-efficient output (axi.md #1): render the tool result as TOON.
	// JSON is the MCP default; TOON goes inside the text payload.
	var body string
	if result.Result != nil {
		if encoded, encErr := toon.Encode(result.Result.Data); encErr == nil {
			body = encoded
		} else {
			raw, _ := json.Marshal(result.Result.Data)
			body = string(raw)
		}
	} else if result.Failure != nil {
		body = fmt.Sprintf("failure: %s — %s", result.Failure.Code, result.Failure.Message)
	}

	// Append suggestions (axi.md #9) so the calling agent sees next moves.
	if result.Result != nil && len(result.Result.Suggestions) > 0 {
		var b strings.Builder
		b.WriteString(body)
		b.WriteString("\n\nsuggested_next:\n")
		for _, s := range result.Result.Suggestions {
			fmt.Fprintf(&b, "  %s — %s\n", s.Action, s.Description)
		}
		body = strings.TrimRight(b.String(), "\n")
	}

	return callToolResult{
		Content: []content{{Type: "text", Text: body}},
		IsError: result.Status == domain.StatusFailed,
	}
}

// contractToJSONSchema converts an axi-go Contract to a JSON Schema fragment
// suitable for MCP's tool.inputSchema.
func contractToJSONSchema(c domain.Contract) map[string]any {
	props := make(map[string]any, len(c.Fields))
	var required []string
	for _, f := range c.Fields {
		prop := map[string]any{}
		if t := fieldType(f.Type); t != "" {
			prop["type"] = t
		}
		if f.Description != "" {
			prop["description"] = f.Description
		}
		if f.Example != nil {
			prop["examples"] = []any{f.Example}
		}
		props[f.Name] = prop
		if f.Required {
			required = append(required, f.Name)
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func fieldType(t string) string {
	// axi uses "string"/"number"/"boolean"/"object"/"array"; JSON Schema uses
	// the same names, so a direct pass-through works.
	return t
}
