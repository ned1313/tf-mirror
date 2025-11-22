# Testing the Server

## Important: Background Process Required

When testing the server in the terminal, you MUST run it as a background process if you want to execute other commands (like curl) in the same terminal session.

### Starting the Server as Background Process

**Option 1: Using -isBackground flag in run_in_terminal**
```powershell
go run cmd/terraform-mirror/main.go
```
Set `isBackground: true` when calling the tool.

**Option 2: Using Start-Process (PowerShell)**
```powershell
Start-Process pwsh -ArgumentList "-NoExit", "-Command", "go run cmd/terraform-mirror/main.go"
```

**Option 3: Building and running in separate window**
```powershell
go build -o terraform-mirror.exe ./cmd/terraform-mirror
Start-Process pwsh -ArgumentList "-NoExit", "-Command", ".\terraform-mirror.exe"
```

### Testing Endpoints

Once the server is running in the background, you can test endpoints:

```powershell
# Health check
curl -s http://localhost:8080/health | jq

# Service discovery
curl -s http://localhost:8080/.well-known/terraform.json | jq

# Provider loading
curl -X POST -F "file=@examples/providers-test.hcl" http://localhost:8080/admin/api/providers/load | jq
```

### Common Mistakes

❌ **DON'T**: Run `go run` with `isBackground: false` and then try to curl in the same terminal
- The terminal will be blocked by the running server
- Any subsequent commands will kill the server

✅ **DO**: Always use `isBackground: true` or start in a separate PowerShell window

### Stopping the Server

Find and stop the process:
```powershell
# Find the process
Get-Process | Where-Object {$_.ProcessName -like "*terraform-mirror*"}

# Stop by process ID
Stop-Process -Id <PID> -Force
```

Or use Ctrl+C in the window where the server is running.
