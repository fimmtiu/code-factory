build:
	go build ./cmd/tickets ./cmd/ticketsd ./cmd/tickets-ui ./cmd/tickets-testdata

test:
	go test ./...

lint:
	go vet ./...
	gofmt -w .

clean:
	rm -f tickets ticketsd tickets-ui tickets-testdata

clean-data:
	rm -rf .tickets

install: build
	GOBIN=$(HOME)/bin go install ./cmd/tickets ./cmd/ticketsd ./cmd/tickets-ui ./cmd/tickets-testdata
	for skill in skills/*; do
		cp -rf $$skill $(HOME)/.claude/skills/
	done

.PHONY: build test lint clean clean-data install
