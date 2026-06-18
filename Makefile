.PHONY: build vet test cross clean windows

GOCMD=/opt/data/go/bin/go
GOBUILD=$(GOCMD) build
GOVET=$(GOCMD) vet

build:
	$(GOBUILD) -o ./cmd/hermes-remote/hermes-remote ./cmd/hermes-remote/
	$(GOBUILD) -o ./cmd/server/server ./cmd/server/

vet:
	$(GOVET) ./...

test:
	$(GOCMD) test ./... -v -count=1

cross:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o ./build/hermes-remote-linux-amd64 ./cmd/hermes-remote/
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o ./build/hermes-remote-linux-arm64 ./cmd/hermes-remote/
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o ./build/hermes-remote-windows-amd64.exe ./cmd/hermes-remote/
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o ./build/hermes-remote-darwin-amd64 ./cmd/hermes-remote/
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o ./build/hermes-remote-darwin-arm64 ./cmd/hermes-remote/

clean:
	rm -rf ./build/
	rm -f ./cmd/hermes-remote/hermes-remote
	rm -f ./cmd/server/server

windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "-s -w" -o ./build/HermesRemote.exe ./cmd/hermes-remote/
