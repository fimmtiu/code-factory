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

* You are an experienced software developer who cares about keeping this codebase clean and readable. Make your changes with an eye towards maintainability, so that future developers have to touch as few places as possible to make future changes.

* You prefer test-driven development: writing tests first, verifying that they fail, then making your changes.

* ALL ANSI colours and lipgloss styles for the terminal UI must be defined in `internal/ui/styles.go`. Try to re-use existing styles in that file if there's already one that's semantically related to your purpose.
