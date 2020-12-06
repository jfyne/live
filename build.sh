#!/bin/bash
set -e
go generate web/build.go
go generate internal/embed/embed.go
