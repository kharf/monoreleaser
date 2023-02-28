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
	go mod download
	go mod verify
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags="-s -w" -v -o build/output/monoreleaser-linux-amd64 cmd/$(cmd)/main.go
	CGO_ENABLED=0 GOARCH=amd64 GOOS=windows go build -ldflags="-s -w" -v -o build/output/monoreleaser-windows-amd64.exe cmd/$(cmd)/main.go
	CGO_ENABLED=0 GOARCH=amd64 GOOS=darwin go build -ldflags="-s -w" -v -o build/output/monoreleaser-darwin-amd64 cmd/$(cmd)/main.go
	CGO_ENABLED=0 GOARCH=arm64 GOOS=darwin go build -ldflags="-s -w" -v -o build/output/monoreleaser-darwin-arm64 cmd/$(cmd)/main.go

.PHONY: clean
clean:
	rm -rf build/output/

.PHONY: test
test:
	go test ./... -v