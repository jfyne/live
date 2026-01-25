# Dependencies

## Core Framework Dependencies (Go)

### WebSocket Communication
- `github.com/coder/websocket v1.8.13` - WebSocket protocol implementation for real-time client-server communication

### Utilities
- `github.com/rs/xid v1.6.0` - Fast unique ID generation for session tracking
- `golang.org/x/net v0.39.0` - HTML parsing utilities for DOM manipulation
- `golang.org/x/time v0.11.0` - Rate limiting support for event throttling/debouncing

### Testing
- `github.com/google/go-cmp v0.7.0` - Deep comparison utilities for test assertions

## Client Library Dependencies (TypeScript/JavaScript)

### Build & Bundle
- `esbuild` - Fast TypeScript compiler and bundler
- `typescript` - Type-safe client library development

### Testing
- `jest` - JavaScript testing framework
- `ts-jest` - TypeScript support for Jest
- `@types/*` - TypeScript type definitions

### Development
- `prettier` - Code formatting
- `npm-run-all` - Parallel script execution

## Standard Library Usage

Heavy reliance on Go stdlib:
- `html/template` - Server-side HTML rendering
- `net/http` - HTTP server and routing (framework is http.Handler compatible)
- `context` - Request scoping and cancellation
- `encoding/json` - WebSocket message serialization
- `sync` - Concurrency primitives (RWMutex for socket management)

## External Service Integrations

**None required by default.** The framework is intentionally storage and service agnostic.

### Optional Integrations
- **PubSub Transport**: Pluggable interface for multi-instance clustering (users can implement with Redis, NATS, etc.)
- **State Store**: Pluggable interface for distributed state (default is in-memory)
