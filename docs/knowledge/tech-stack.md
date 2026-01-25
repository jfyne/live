# Tech Stack

## Languages

- **Go 1.23+**: Backend framework implementation with toolchain 1.24.0
- **TypeScript/JavaScript**: Browser client library for WebSocket communication and DOM patching

## Key Dependencies

### Go Dependencies (Minimal, stdlib-focused)
- `github.com/coder/websocket v1.8.13` - WebSocket protocol implementation
- `github.com/rs/xid v1.6.0` - Unique ID generation for sessions
- `golang.org/x/net v0.39.0` - HTML parsing utilities
- `golang.org/x/time v0.11.0` - Rate limiting support
- `github.com/google/go-cmp v0.7.0` - Testing comparisons

### TypeScript/JavaScript Dependencies
- **esbuild** - Fast TypeScript bundler
- **Jest** + **ts-jest** - Testing framework
- **TypeScript** - Type-safe client library
- **Prettier** - Code formatting

## Build Tools

- **go generate** - Embeds web assets into Go binary
- **esbuild** - Bundles TypeScript client library
- **just** - Build automation (justfile)
- **embedmd** - Embeds code examples in README

## External Integrations

None required - framework is storage-agnostic. Applications using Live choose their own database and external services. Optional PubSub interface available for multi-instance clustering.
