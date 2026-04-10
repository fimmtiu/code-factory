build:
	go build ./cmd/cf-tickets
	go build ./cmd/cf-testdata
	go build ./cmd/code-factory

test:
	go test ./...

lint:
	go vet ./...
	gofmt -w .

clean:
	rm -f cf-tickets cf-testdata code-factory

clean-data:
	rm -rf .code-factory

INSTALL_DIR ?= $(HOME)/bin

install: build
	GOBIN=$(INSTALL_DIR) go install ./cmd/cf-tickets ./cmd/cf-testdata ./cmd/code-factory
	for skill in skills/*; do \
		cp -rf $$skill $(HOME)/.claude/skills/; \
	done

.PHONY: build test lint clean clean-data install
