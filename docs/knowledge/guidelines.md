# Guidelines

## Patterns to Prefer

### Minimal Dependencies
- Prefer standard library over third-party packages
- Only add dependencies when they provide significant value
- Keep the framework lightweight and easy to understand

### Stdlib-First Approach
- Use `html/template` for rendering
- Use standard `testing` package
- Leverage `net/http` compatibility
- Use `context.Context` for request scoping

### Explicit Error Handling
- Return errors up the call stack
- Provide context when wrapping errors
- Let callers decide how to handle errors
- Never panic in library code - return errors instead

### Clear Interfaces
- Keep interfaces small and focused
- Use `net/http` compatible types where possible
- Design for composability with middleware

## Patterns to Avoid

### Heavy Dependencies
- Don't add dependencies for simple tasks that stdlib can handle
- Avoid frameworks that pull in large dependency trees
- Keep go.mod minimal

### Panic in Library Code
- Library code should never panic
- Return errors for all failure cases
- Only use panic for truly unrecoverable programmer errors (e.g., nil pointer bugs)

### Implicit Errors
- Don't silently ignore errors
- Don't use sentinel values (like -1, "", nil) to indicate errors when an error type is clearer
- Always check and propagate errors appropriately

## Error Handling

- Return errors explicitly from all functions that can fail
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Let the caller decide whether to log, retry, or propagate
- Use standard error patterns - avoid custom error types unless necessary
