.PHONY: build test verify generate release-check

build:
	go build -o ./bin/redmine-cli ./cmd/redmine-cli

test:
	go test -race ./...

generate:
	go generate ./...

verify:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go build ./...
	go test -race ./...

release-check: verify
	go mod verify
	go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
	actionlint -no-color
	./tools/release/test-source-bundle.sh
