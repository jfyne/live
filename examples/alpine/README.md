# Alpine example

Demonstrates importing the `@jfyne/live` package, integrating with alpine.js and compiling it with `esbuild`. This
bundles `index.js` into `main.js` which is then embedded and served by the Go binary.

## Setup

```bash
cd examples/alpine
npm install
npm run prepare
cd ..
go run ./alpine
```
