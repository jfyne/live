# Project: Live

A real-time web framework for building interactive UIs entirely in Go.

## Purpose

Live is an alternative to React, Vue, and Angular that enables developers to build interactive web applications using only Go and its templates. Inspired by Phoenix LiveView, it provides server-rendered HTML with WebSocket-based real-time updates and automatic DOM diffing, eliminating the need for frontend JavaScript frameworks.

## Key Concepts

- **Handler**: The core controller that manages mount, render, and event handling logic
- **Socket**: Maintains connection state and assigns (model data) between client and server
- **Events**: User interactions (clicks, form changes, etc.) sent from client to server via WebSocket
- **DOM Diffing**: Server calculates minimal DOM patches and pushes them to browser for efficient updates

## Users

Go developers building interactive web applications who want to avoid JavaScript frameworks and build UIs entirely in Go, leveraging their existing Go skills and templates.
