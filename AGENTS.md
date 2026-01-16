# Agent Guidelines for Lemme

This file provides essential information for coding agents working on this repository.

## Essential Commands

```bash
# Build & Test
make build                    # Build binary
make test                     # Run all tests
go test ./cmd                 # Test specific package
go test ./cmd -run TestName   # Run single test (add -v for verbose)
make test-coverage            # Generate coverage report
make test-race               # Run with race detector

# Linting & Quality
make check                   # Run format, vet, and tests
make lint                    # Run golangci-lint
go fmt ./...                 # Format code

# Cleanup
make clean                   # Remove build artifacts
```

## Code Style Guidelines

### Import Organization
```go
import (
    "fmt"           // standard library
    "os"

    "github.com/charmbracelet/bubbletea"  // external deps
    "github.com/spf13/cobra"

    "github.com/nchapman/lemme/internal/config"  // internal packages
    "github.com/nchapman/lemme/internal/ui"
)
```
Standard library first, then external dependencies, then internal packages. Separate groups with blank lines.

### Naming Conventions
- Exported: `PascalCase` for types, functions, constants
- Private: `camelCase` for types, functions, variables
- Constants: `PascalCase`
- Files: lowercase, snake_case for tests (`cmd/run_test.go`)

### Structs & Types
```go
type Config struct {
    FieldName   string  `yaml:"field_name"`  // snake_case for YAML
    AnotherField int     `yaml:"another_field"`
}

func NewConfig() *Config {
    return &Config{
        FieldName:   "default",
        AnotherField: 42,
    }
}
```
Use YAML struct tags for config structs. Provide constructor functions with `New` prefix.

### Error Handling
```go
// Always check errors
data, err := os.ReadFile(path)
if err != nil {
    return fmt.Errorf("failed to read file: %w", err)
}

// Wrap errors with context
if err := doSomething(); err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Use os.Exit(1) for CLI errors with helpful messages
if err != nil {
    fmt.Printf("%s Failed to load config: %v\n", ui.ErrorMsg("Error:"), err)
    os.Exit(1)
}
```
Always check errors. Use `%w` to wrap errors. Provide context in error messages. Use `os.Exit(1)` in CLI commands with formatted error output.

### Functions & Methods
```go
// Small, focused functions
func process(input string) (string, error) {
    return strings.ToUpper(input), nil
}

// Method on struct
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    return m, nil
}
```
Keep functions small (ideally < 30 lines). Use pointer receivers for structs that need mutation.

### Testing
```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"test case", "input", "output", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := function(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if result != tt.expected {
                t.Errorf("result = %v, want %v", result, tt.expected)
            }
        })
    }
}
```
Use table-driven tests with subtests. Name test functions `TestFunctionName`. Check both expected errors and return values.

### Bubbletea TUI Pattern
```go
type model struct {
    viewport  viewport.Model
    textarea  textarea.Model
    messages  []string
    err       error
    generating bool
}

func initialModel() model {
    return model{
        messages: []string{},
        // initialize fields
    }
}

func (m model) Init() tea.Cmd {
    return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.Type {
        case tea.KeyCtrlC:
            return m, tea.Quit
        }
    case customMsg:
        // handle custom message
    }
    return m, nil
}

func (m model) View() string {
    return m.viewport.View() + "\n" + m.textarea.View()
}
```
TUI models follow Bubbletea's Model interface: `Init()`, `Update()`, `View()`.

### API Client Pattern
```go
type APIClient struct {
    baseURL string
    client  *http.Client
}

func NewAPIClient(host string, port int) *APIClient {
    return &APIClient{
        baseURL: fmt.Sprintf("http://%s:%d", host, port),
        client:  &http.Client{},
    }
}

func (api *APIClient) Method() error {
    url := fmt.Sprintf("%s/endpoint", api.baseURL)
    resp, err := api.client.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("request failed: HTTP %d", resp.StatusCode)
    }

    return nil
}
```
API clients use constructor pattern with `New` prefix. Always close response bodies. Check HTTP status codes.

### No Comments
Do not add comments unless absolutely necessary. The code should be self-documenting through clear naming and structure.

### File Organization
- `cmd/` - CLI commands (run, pull, list, etc.)
- `internal/` - Internal packages (config, server, ui, etc.)
- `main.go` - Application entry point

## Important Notes

- Always run `make check` and `make lint` before committing
- Binary is built to `build/lemme` via `make build`
- Use `.gitignore` to exclude build artifacts
