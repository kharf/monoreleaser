.PHONY: lint
lint:
	golangci-lint run -c .golangci.yaml

.PHONY: verify
verify:
	go mod download && go mod verify

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: compile
compile:
	go mod download && go mod verify && \
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags="-s -w" -v -o build/output/$(cmd) cmd/$(cmd)/main.go

.PHONY: clean
clean:
	rm -rf build/output/

.PHONY: test
test:
	go test ./... -v