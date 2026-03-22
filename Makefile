build:
	go build ./cmd/tickets ./cmd/ticketsd ./cmd/tickets-ui ./cmd/gen-testdata

test:
	go test ./...

lint:
	go vet ./...
	gofmt -w .

clean:
	rm -f tickets ticketsd tickets-ui gen-testdata

data:
	./gen-testdata

clean-data:
	rm -rf .tickets

.PHONY: build test lint clean data clean-data
