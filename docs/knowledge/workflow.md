# Workflow

## Testing Approach

- Write tests alongside implementation
- Implement feature, add tests as you go, iterate until complete
- Use Go's standard testing package with race detection (`go test -race ./...`)
- Use Jest for TypeScript/JavaScript tests

## Commit Style

**Conventional commits format:**

- `feat:` - New features
- `feat(scope):` - New features in specific area (e.g., `feat(uploads):`)
- `fix:` - Bug fixes
- `fix(scope):` - Bug fixes in specific area (e.g., `fix(uploads):`)
- `chore:` - Maintenance tasks
- `docs:` - Documentation changes
- `test:` - Test additions or changes

Example: `feat: support configuring the max message size`

## Code Review Process

- Create pull request for all changes
- GitHub code review required
- Address feedback and iterate
- Merge when approved

## Pre-Merge Requirements

- [ ] All Go tests pass with race detection
- [ ] All TypeScript/Jest tests pass
- [ ] CI passes on all platforms (Linux, macOS, Windows)
- [ ] CI passes on all supported Node versions
- [ ] No new linting errors
