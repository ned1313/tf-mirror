# Windows-Specific Requirements

## File Path Handling

**IMPORTANT**: On Windows, all file paths used as command-line arguments must be enclosed in double quotes.

### Examples

❌ **Incorrect** (will fail on Windows):
```powershell
go test -config=config/test.hcl
./terraform-mirror -config config.hcl
```

✅ **Correct** (works on Windows):
```powershell
go test -config="config/test.hcl"
./terraform-mirror -config "config.hcl"
```

### When This Applies

- All terminal commands that accept file paths as arguments
- PowerShell commands
- Any commands run via `run_in_terminal` tool
- Commands in Makefile (when run on Windows)
- Docker commands with volume mounts (paths with spaces or special characters)

### Reason

Windows paths can contain:
- Spaces (e.g., `C:\Program Files\`)
- Special characters
- Backslashes as path separators

Without quotes, these paths will be parsed incorrectly by the shell.

## Other Windows Considerations

1. **Path Separators**: Use `filepath.Join()` in Go code for cross-platform compatibility
2. **Line Endings**: Git should handle CRLF/LF conversion automatically
3. **Case Sensitivity**: Windows filesystems are case-insensitive but case-preserving
4. **PowerShell Syntax**: Different from bash (e.g., environment variables use `$env:VAR` not `$VAR`)

---

**Date Created**: November 20, 2025
**Last Updated**: November 20, 2025
