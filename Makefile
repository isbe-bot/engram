.PHONY: build test run fmt vet tidy clean

build:
	@mkdir -p bin
	go build -o ./bin/engramd ./cmd/engramd
	go build -o ./bin/engramctl ./cmd/engramctl

test:
	go test ./...

run:
	go run ./cmd/engramd --config ./configs/example.yaml

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf ./bin
