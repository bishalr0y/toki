build:
    go build -o bin/toki ./cmd/toki

test:
    go test -race -count=1 ./...

lint:
    golangci-lint run ./...

run: build
    ./bin/toki

clean:
    rm -rf bin/
