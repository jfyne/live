help:
    just -l

test: _test_go _test_js

_test_go:
    go vet ./...
    go test ./...

_test_js:
    cd web && npm run test
