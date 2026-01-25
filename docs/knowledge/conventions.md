# Conventions

## File Naming

- Test files: `*_test.go` suffix, same package as source
- Errors: Centralized in `errors.go` with `var ErrXxx = errors.New(...)`
- Features organized by domain (e.g., `/page` for component features)
- Descriptive lowercase names: `handler.go`, `socket.go`, `render.go`

## Test Patterns

- Tests located alongside source files (no separate test directory)
- Test functions: `func TestHandlerName(t *testing.T)`
- Example tests: `func Example()` for documentation
- Simple assertions: direct comparison + `t.Fatal()` or `t.Error()`
- Table-driven tests for complex scenarios

## Import Organization

**Three groups, separated by blank lines:**

1. Standard library imports (alphabetical)
2. External packages (alphabetical)
3. Local package imports (fully qualified)

Example:
```go
import (
    "context"
    "fmt"

    "github.com/coder/websocket"

    "github.com/jfyne/live"
)
```

## Error Handling Patterns

- **Predefined errors**: `var ErrXxx = errors.New("message")` in `errors.go`
- **Wrap with context**: `fmt.Errorf("context: %w", err)`
- **Custom errors**: Implement `Error()` and `Unwrap()` methods
- **Always check**: `if err != nil` without silent failures
- **No panics**: Library code returns errors instead

## Code Organization

- **Options pattern**: `TypeConfig func(t *Type) error` for flexible configuration
- **Builder functions**: `WithField(value) TypeConfig` for clear initialization
- **Lifecycle hooks**: `Mount`, `Render`, `Event`, `Self`, `Unmount` methods
- **Default implementations**: Return nil for no-op handlers
- **Dependency injection**: Pass via struct fields or function parameters
- **Scoped events**: Use component ID prefixes to avoid naming conflicts
