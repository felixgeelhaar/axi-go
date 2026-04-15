// Package api provides a Gin-like HTTP API layer for axi-go, built on net/http.
package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// HandlerFunc is a Gin-style handler that receives a Context.
type HandlerFunc func(c *Context)

// Context wraps an HTTP request/response with Gin-like convenience methods.
type Context struct {
	Request *http.Request
	Writer  http.ResponseWriter
	params  map[string]string
	written bool
	status  int
}

// Param returns a path parameter by name (Gin-style).
func (c *Context) Param(name string) string {
	return c.params[name]
}

// JSON serializes the value as JSON and writes it with the given status code.
func (c *Context) JSON(code int, obj any) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(code)
	c.status = code
	c.written = true
	_ = json.NewEncoder(c.Writer).Encode(obj)
}

// ShouldBindJSON decodes the request body into the given struct.
func (c *Context) ShouldBindJSON(obj any) error {
	return json.NewDecoder(c.Request.Body).Decode(obj)
}

// Status sets the HTTP status code without a body.
func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
	c.status = code
	c.written = true
}

// Engine is a Gin-like HTTP router built on net/http.ServeMux.
type Engine struct {
	mux        *http.ServeMux
	middleware []HandlerFunc
}

// New creates a new Engine.
func New() *Engine {
	return &Engine{
		mux: http.NewServeMux(),
	}
}

// Use adds middleware that runs before every handler.
func (e *Engine) Use(middleware ...HandlerFunc) {
	e.middleware = append(e.middleware, middleware...)
}

// Group creates a route group with a common prefix.
func (e *Engine) Group(prefix string) *RouterGroup {
	return &RouterGroup{engine: e, prefix: strings.TrimRight(prefix, "/")}
}

// GET registers a handler for GET requests.
func (e *Engine) GET(pattern string, handler HandlerFunc) {
	e.handle("GET", pattern, handler)
}

// POST registers a handler for POST requests.
func (e *Engine) POST(pattern string, handler HandlerFunc) {
	e.handle("POST", pattern, handler)
}

// PUT registers a handler for PUT requests.
func (e *Engine) PUT(pattern string, handler HandlerFunc) {
	e.handle("PUT", pattern, handler)
}

// DELETE registers a handler for DELETE requests.
func (e *Engine) DELETE(pattern string, handler HandlerFunc) {
	e.handle("DELETE", pattern, handler)
}

func (e *Engine) handle(method, pattern string, handler HandlerFunc) {
	// Go 1.22+ ServeMux supports "METHOD /path" and {param} syntax.
	muxPattern := method + " " + pattern
	middleware := e.middleware
	e.mux.HandleFunc(muxPattern, func(w http.ResponseWriter, r *http.Request) {
		ctx := &Context{
			Request: r,
			Writer:  w,
			params:  extractParams(r, pattern),
		}
		for _, mw := range middleware {
			mw(ctx)
			if ctx.written {
				return
			}
		}
		handler(ctx)
	})
}

// Handler returns the underlying http.Handler.
func (e *Engine) Handler() http.Handler {
	return e.mux
}

// Run starts the HTTP server on the given address.
func (e *Engine) Run(addr string) error {
	return http.ListenAndServe(addr, e.mux)
}

// RouterGroup groups routes under a common prefix.
type RouterGroup struct {
	engine *Engine
	prefix string
}

// GET registers a handler for GET requests within this group.
func (g *RouterGroup) GET(pattern string, handler HandlerFunc) {
	g.engine.GET(g.prefix+pattern, handler)
}

// POST registers a handler for POST requests within this group.
func (g *RouterGroup) POST(pattern string, handler HandlerFunc) {
	g.engine.POST(g.prefix+pattern, handler)
}

// PUT registers a handler for PUT requests within this group.
func (g *RouterGroup) PUT(pattern string, handler HandlerFunc) {
	g.engine.PUT(g.prefix+pattern, handler)
}

// DELETE registers a handler for DELETE requests within this group.
func (g *RouterGroup) DELETE(pattern string, handler HandlerFunc) {
	g.engine.DELETE(g.prefix+pattern, handler)
}

// extractParams pulls path parameters from the request using Go 1.22+ PathValue.
func extractParams(r *http.Request, pattern string) map[string]string {
	params := make(map[string]string)
	// Parse {param} placeholders from the pattern.
	for _, segment := range strings.Split(pattern, "/") {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := segment[1 : len(segment)-1]
			if val := r.PathValue(name); val != "" {
				params[name] = val
			}
		}
	}
	return params
}
