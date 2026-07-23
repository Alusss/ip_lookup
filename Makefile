.PHONY: build test clean run lint docker

APP_NAME = ip-lookup
BUILD_FLAGS = -trimpath -ldflags="-s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)"

build:
	cd backend && CGO_ENABLED=0 go build $(BUILD_FLAGS) -o $(APP_NAME) .

test:
	cd backend && go test ./... -v -count=1

clean:
	rm -f backend/$(APP_NAME)
	rm -f backend/$(APP_NAME).exe

run: build
	cd backend && ./$(APP_NAME) -config config.yaml

lint:
	golangci-lint run ./backend/...

docker:
	docker build -t $(APP_NAME):latest -f docker/Dockerfile .

.PHONY: verify
verify:
	bash scripts/verify.sh
