#!/bin/bash
set -e
if ! [ -d web/node_modules ]; then
    cd web && npm install
    cd -
fi
go generate web/build.go
go generate internal/embed/embed.go
if ! command -v embedmd &> /dev/null
then
    GO111MODULE=off go get github.com/campoy/embedmd
fi
embedmd -w README.md
