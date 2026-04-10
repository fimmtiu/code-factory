# AGENTS.md

## Common `make` commands

```
make build    # Build all three binaries
make test     # Run the test suite
make lint     # Run go vet and gofmt
make clean    # Remove built binaries
make install  # Build, install to bin directory, and install skills
```

`make lint` MUST be run before committing a change.

## Codebase rules

* ALL ANSI colours and lipgloss styles for the terminal UI must be defined in `internal/ui/colours.go`. Try to re-use existing styles in that file if there's already one that suits your purpose.
