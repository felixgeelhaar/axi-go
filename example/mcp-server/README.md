# MCP server sketch (copy/vendor only)

Exposes an axi-go kernel as a Model Context Protocol tool provider over
stdio — JSON-RPC 2.0, newline-delimited, **stdlib only**.

## Status

**Not a supported axi-go package.** No SemVer, no stability guarantee.
Copy or vendor this tree into your own module if you need an MCP adapter.

A first-party MCP package in axi-go core is **declined** (see
[`docs/ROADMAP.md`](../../docs/ROADMAP.md) and [`docs/backlog.md`](../../docs/backlog.md)):
axi-go stays zero-deps and does not import an MCP schema.

## Run

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | go run ./example/mcp-server
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | go run ./example/mcp-server
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo.upper","arguments":{"text":"hello"}}}' | go run ./example/mcp-server
```

Tests: `go test ./example/mcp-server/ -count=1`
