#!/bin/bash
set -e
go generate web/build.go
go generate internal/embed/embed.go
if ! command -v embedmd &> /dev/null
then
    GO111MODULE=off go get github.com/campoy/embedmd
fi
embedmd -w README.md
