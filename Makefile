BINARY_NAME=rcv-app
MAIN_PATH=./cmd/server/main.go
AIR_CONFIG=$(if $(filter Windows_NT,$(OS)),.air.windows.toml,.air.toml)

.PHONY: build run dev tidy clean

tidy:
	go mod tidy

build:
	go build -o ${BINARY_NAME} ${MAIN_PATH}

run:
	go run ${MAIN_PATH}

dev: tidy
	air -c $(AIR_CONFIG)

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