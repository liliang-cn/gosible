# Enhanced Features Demo

This example demonstrates all the enhanced gosinble features working together:

## Features Demonstrated

- **File Transfer Progress**: Real-time progress tracking for copy operations
- **WebSocket Streaming**: Live updates broadcasted to web clients  
- **Comprehensive Logging**: Multi-output logging with filtering and structured data
- **Integration**: Shows how all features work together seamlessly

## Running the Demo

```bash
go run examples/enhanced-features-demo/main.go
```

## WebSocket Client Test

Open browser console and connect to the WebSocket server:

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received:', data.type, data);
};
ws.send(JSON.stringify({
  type: 'subscribe',
  data: { event_types: ['stream_event', 'progress', 'connection'] }
}));
```

## Output Files

- `./demo_output.txt` - Enhanced copy output file
- `./demo_logs.json` - Comprehensive structured logs
- `./demo_output.txt.backup.*` - Backup files created during copy

## Expected Output

```
🚀 Gosinble Enhanced Features Demo
===================================
Setting up comprehensive logging...
Starting WebSocket server for real-time updates...

📁 Demo 1: Enhanced File Transfer with Progress
  🔄 Copying file with progress tracking...
  ✅ Copy completed successfully!
  📊 File size: 423 bytes
  ⏱️  Transfer time: 12.34ms
  🚀 Average speed: 34.3 KB/s

🌐 Demo 2: WebSocket Real-Time Streaming
  📡 Broadcasting events to WebSocket clients...
  ✅ WebSocket streaming demo completed!

📝 Demo 3: Comprehensive Logging Integration
  📄 Logging various message levels...
  ✅ Logging integration demo completed!

✅ Demo completed successfully!
```

## Key Integration Points

1. **WebSocket + Progress**: File transfer progress is broadcast to web clients
2. **Logging + Events**: All operations are logged with structured data
3. **Progress + Performance**: Real-time metrics and performance tracking
4. **Error Handling**: Comprehensive error handling across all components