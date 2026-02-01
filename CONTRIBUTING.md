# Contributing to slack-cli

Thank you for your interest in contributing to slack-cli! This document provides guidelines and information for contributors.

## Code of Conduct

Be respectful, inclusive, and professional in all interactions.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/slack-cli.git`
3. Create a feature branch: `git checkout -b feature/my-new-feature`
4. Make your changes
5. Test thoroughly
6. Commit and push
7. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.21 or higher
- A Slack workspace for testing

### Building

```bash
go build -o slack-cli .
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/slack/...
```

### Code Style

- Follow standard Go conventions
- Use `gofmt` to format code
- Run `go vet` to check for issues
- Keep functions focused and well-documented
- Write tests for new features

### Commit Messages

Use conventional commit format:

- `feat: add new feature`
- `fix: resolve bug`
- `docs: update documentation`
- `test: add tests`
- `refactor: improve code structure`
- `chore: update dependencies`

## Pull Request Process

1. **Update documentation** - If you add features, update README.md and DESIGN.md
2. **Add tests** - New features should include tests
3. **Check formatting** - Run `gofmt -s -w .`
4. **Verify builds** - Ensure `go build ./...` succeeds
5. **Run tests** - Ensure `go test ./...` passes
6. **Update CHANGELOG** - Add entry for your changes (if applicable)
7. **Describe changes** - Provide clear PR description

### PR Title Format

```
<type>: <short description>

Examples:
feat: add message threading support
fix: resolve cache corruption issue
docs: improve installation instructions
```

## Feature Requests and Bug Reports

### Bug Reports

Include:
- Go version (`go version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs or error messages

### Feature Requests

Include:
- Use case description
- Proposed solution
- Alternative solutions considered
- Impact on existing features

## Project Structure

```
slack-cli/
├── cmd/                  # CLI commands (Cobra)
│   ├── root.go          # Root command
│   ├── config.go        # Config commands
│   ├── messages.go      # Message commands
│   └── ...
├── internal/            # Internal packages
│   ├── slack/          # Slack API client
│   ├── config/         # Configuration management
│   ├── cache/          # Caching layer
│   ├── output/         # Output formatting
│   └── ...
├── docs/               # Documentation
├── main.go            # Entry point
└── README.md          # Project documentation
```

## Testing Guidelines

### Unit Tests

- Test individual functions and methods
- Mock external dependencies (Slack API)
- Use table-driven tests for multiple scenarios
- Aim for >80% coverage on new code

Example:

```go
func TestChannelResolver(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        want     string
        wantErr  bool
    }{
        {
            name:    "valid channel name",
            input:   "#general",
            want:    "C123ABC",
            wantErr: false,
        },
        // more cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### Integration Tests

- Test with real Slack API (when possible)
- Use environment variables for credentials
- Skip if credentials not available

```go
func TestSlackIntegration(t *testing.T) {
    if os.Getenv("SLACK_USER_TOKEN") == "" {
        t.Skip("SLACK_USER_TOKEN not set")
    }
    // test implementation
}
```

## Documentation

- Keep README.md up to date
- Document all public functions and types
- Provide examples for complex features
- Update DESIGN.md for architectural changes

## Questions?

- Open an issue for questions
- Check existing issues first
- Be specific and provide context

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
