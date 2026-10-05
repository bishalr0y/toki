# Stamped into `toki --version`. Override per build: `just build version=1.2.3`.
version := "dev"

build:
    go build -ldflags "-X main.version={{version}}" -o bin/toki ./cmd/toki

test:
    go test -race -count=1 ./...

lint:
    golangci-lint run ./...

# Install golangci-lint v2; a v1 build cannot target this module's Go version.
lint-install:
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
    @echo "installed to $(go env GOPATH)/bin — ensure that directory is on your PATH"

run: build
    ./bin/toki

clean:
    rm -rf bin/
