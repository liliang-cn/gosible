# gosible Examples

This directory contains comprehensive examples demonstrating gosible's enhanced features including file transfer progress, WebSocket streaming, logging integration, and step tracking.

## Available Examples

### 1. Enhanced Features Demo

**Location**: `examples/enhanced-features-demo/`
**Run**: `go run examples/enhanced-features-demo/main.go`

A comprehensive demonstration of all enhanced gosible features working together:

- File transfer progress tracking with real-time callbacks
- WebSocket streaming for live web dashboard updates
- Multi-output comprehensive logging (file, console, memory)
- Performance metrics and backup functionality

### 2. Step Tracking Integration

**Location**: `examples/step-tracking-integration/`
**Run**: `go run examples/step-tracking-integration/main.go`

Demonstrates integration between step tracking and enhanced features:

- Multi-step deployment simulation (8 steps)
- Real-time WebSocket broadcasting of step events
- Comprehensive step lifecycle logging with metadata
- Overall and per-step progress tracking

## Prerequisites

Ensure you have the required dependencies:

```bash
go get github.com/gorilla/websocket
```

## Quick Start

Run all examples to see the enhanced features in action:

```bash
# Enhanced features demo with WebSocket server
go run examples/enhanced-features-demo/main.go

# Step tracking integration demo
go run examples/step-tracking-integration/main.go
```

## WebSocket Testing

For the enhanced features demo, connect to the WebSocket server:

1. Run the demo: `go run examples/enhanced-features-demo/main.go`
2. Open browser console and connect:

```javascript
const ws = new WebSocket("ws://localhost:8080/ws");
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log("Received:", data.type, data);
};
ws.send(
  JSON.stringify({
    type: "subscribe",
    data: { event_types: ["stream_event", "progress", "connection"] },
  })
);
```

## Output Files

Running the examples will create:

- `./demo_output.txt` - Enhanced copy output
- `./demo_logs.json` - Structured logging output
- `./demo_output.txt.backup.*` - Backup files

## Key Features Demonstrated

### File Transfer Progress

- Real-time progress callbacks with percentage and timing
- Human-readable transfer speeds (KB/s, MB/s)
- Context cancellation support
- Error handling and recovery

### WebSocket Streaming

- Production-ready server with client management
- Real-time event broadcasting to web clients
- Client subscription filtering
- Concurrent client support (100+ clients tested)

### Comprehensive Logging

- Multi-output logging (file, console, memory)
- Structured JSON logging with filtering
- Async processing with configurable buffering
- Integration with stream events and progress tracking

### Step Tracking Integration

- Step lifecycle management (start, progress, completion)
- Performance timing and success rate tracking
- WebSocket broadcasting of step events
- Comprehensive metadata logging

## Architecture Highlights

- **Interface-Based Design**: Clean abstractions following Go best practices
- **Concurrent Safety**: Thread-safe operations with proper synchronization
- **Error Handling**: Comprehensive error wrapping and context propagation
- **Performance Optimized**: <5% overhead for progress tracking
- **Production Ready**: Graceful shutdown, resource cleanup, observability

## Real-World Use Cases

### OBFY Deployment Platform

- Live deployment dashboards with step-by-step progress
- Audit logging for compliance and troubleshooting
- Performance monitoring with transfer metrics
- Error diagnosis with detailed operation context

### CI/CD Integration

- Pipeline visualization with real-time updates
- Build artifact transfer with progress tracking
- Deployment monitoring across multiple environments
- Centralized logging for build and deployment analysis

### Operations Automation

- Infrastructure deployments with coordination
- Configuration management with change tracking
- Real-time monitoring dashboards
- Incident response with detailed operation logs

These examples provide a complete foundation for integrating gosible's enhanced features into production systems with enterprise-grade visibility and control.
