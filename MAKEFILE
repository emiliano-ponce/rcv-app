BINARY_NAME=rcv-app
MAIN_PATH=./cmd/server/main.go

.PHONY: build run dev tidy clean

tidy:
	go mod tidy

build:
	go build -o ${BINARY_NAME} ${MAIN_PATH}

run:
	go run ${MAIN_PATH}

dev: tidy
	air -c .air.toml

clean:
	go clean
	rm -f ${BINARY_NAME}

fmt:
	gofmt -s -w .

lint:
	staticcheck ./...

test:
	go test ./...

check: fmt lint