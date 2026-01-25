# Architecture

## Directory Structure

| Directory | Purpose |
|-----------|---------|
| `/` (root) | Core framework implementation (engine, handlers, sockets, rendering) |
| `/page` | Component API for reusable UI elements with lifecycle hooks |
| `/web` | TypeScript/JavaScript client library for browser |
| `/web/src` | TypeScript source (socket, events, DOM patching) |
| `/web/dist` | Compiled JavaScript output |
| `/examples` | Reference implementations (todo, chat, buttons, clock, etc.) |

## Entry Points

### For Framework Users
- `NewHttpHandler(ctx, handler)` - Creates HTTP engine serving both HTTP and WebSocket
- `NewHandler(configs...)` - Defines handler with mount, render, and event callbacks
- `h.HandleEvent(name, func)` - Registers client event handlers (clicks, forms)
- `h.HandleSelf(name, func)` - Registers server-to-client event handlers
- `h.HandleParams(func)` - Handles URL parameter changes

### Socket API (available in handlers)
- `s.Assigns()` / `s.Assign(data)` - State management
- `s.Send(event, data)` - Send events to client
- `s.Broadcast(event, data)` - Broadcast to all connected sockets

## Key Modules

- **Engine** (`engine.go`) - Manages socket lifecycle, routes HTTP/WebSocket, orchestrates mount→render→event flow
- **Socket** (`socket.go`) - Represents one client connection with state, render tree, and message buffering
- **Render** (`render.go`, `diff.go`) - Parses HTML templates and computes minimal DOM patches
- **State Store** (`socketstate.go`) - Pluggable storage with TTL-based garbage collection
- **PubSub** (`pubsub.go`) - Pluggable broadcast mechanism for multi-instance deployments
- **Uploads** (`upload.go`) - File upload handling with progress and validation
- **Components** (`page/component.go`) - Reusable UI components with isolated state
