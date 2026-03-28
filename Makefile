build:
	go build ./cmd/tickets
	go build ./cmd/tickets-testdata
	go build ./cmd/code-factory

test:
	go test ./...

lint:
	go vet ./...
	gofmt -w .

clean:
	rm -f tickets tickets-testdata code-factory

clean-data:
	rm -rf .tickets

install: build
	GOBIN=$(HOME)/bin go install ./cmd/tickets ./cmd/tickets-testdata ./cmd/code-factory
	for skill in skills/*; do \
		cp -rf $$skill $(HOME)/.claude/skills/; \
	done
	cp rules/* $(HOME)/.cursor/rules/

.PHONY: build test lint clean clean-data install
