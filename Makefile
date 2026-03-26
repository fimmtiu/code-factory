build:
	go build ./cmd/tickets ./cmd/tickets-testdata

test:
	go test ./...

lint:
	go vet ./...
	gofmt -w .

clean:
	rm -f tickets tickets-testdata

clean-data:
	rm -rf .tickets

install: build
	GOBIN=$(HOME)/bin go install ./cmd/tickets ./cmd/tickets-testdata
	for skill in skills/*; do \
		cp -rf $$skill $(HOME)/.claude/skills/; \
	done

.PHONY: build test lint clean clean-data install
