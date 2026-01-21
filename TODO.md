# Code Improvement Plan

Based on comprehensive code review. Completed items marked with ~~strikethrough~~.

## High Priority

### ~~1. Add TUI Component Tests~~ Done
- [x] `internal/tui/components/completions_test.go` - Completion filtering, navigation, state
- [x] `internal/tui/components/input_test.go` - Input state, completion triggering logic
- [x] `internal/tui/components/messages_test.go` - Streaming state machine, message management

## Medium Priority

### 2. Consolidate Style Definitions
`internal/ui/style.go` and `internal/tui/styles/theme.go` define similar colors/styles.

- [ ] Create shared `internal/styles/` package
- [ ] Migrate CLI styles to use shared package
- [ ] Migrate TUI styles to use shared package
- [ ] Remove duplicate definitions

### ~~3. Extract HTTP Error Helper~~ Done
- [x] Created `checkResponse(resp, operation)` helper in `internal/server/api.go`
- [x] Replaced all duplicate HTTP error handling patterns
- [x] Added error context wrapping throughout api.go

### ~~4. Fix Spinner Issues~~ Done
- [x] Removed unused `s.model` field from `internal/ui/spinner.go`

### ~~5. Move Shared Types~~ Done
- [x] Moved `ModelInfo` to `cmd/types.go`

### ~~6. Add Options Resolver Tests~~ Done
- [x] Added comprehensive table-driven tests in `internal/options/resolver_test.go`

### ~~7. Cache Markdown Renderers~~ Done
- [x] Added `sync.Map` caching by width in `internal/tui/styles/markdown.go`

## Low Priority

### 8. Remove Direct fmt.Print from Library Code
`internal/llama/binary.go` prints directly to stdout during downloads.

- [ ] Accept callback or use logger interface

### 9. Refactor ui.Fatal
`internal/ui/style.go` calls `os.Exit(1)`, making it untestable.

- [ ] Consider alternative patterns (return errors, use cobra's RunE)

### ~~10. Log Ignored Errors~~ Done
- [x] `internal/proxy/manager.go` - TouchLastUsed logs at debug level
- [x] `internal/proxy/server.go` - JSON encoding logs at debug level

### ~~11. Consolidate Editor Opening Code~~ Done
- [x] Extracted shared `openInEditor()` in `cmd/config.go`
- [x] Updated `cmd/persona.go` to use shared function

### ~~12. Remove Unused Code~~ Done
- [x] Removed `normalStyle` from `internal/ui/style.go`

### ~~13. Use Shared Version Constant~~ Done
- [x] Created `internal/version/version.go` package
- [x] Updated all User-Agent headers to use `version.UserAgent()`

### ~~14. Add Missing Error Context~~ Done
- [x] Wrapped errors with `fmt.Errorf("context: %w", err)` throughout `internal/server/api.go`
