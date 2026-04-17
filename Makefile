# meet Makefile.
#
# Local dev targets only. Deployment is handled by the hetzner repo's
# deploy-app command.

APP          := meet
MAIN_PKG     := ./cmd/meet
BUILD_OUTPUT := ./bin/$(APP)

.PHONY: build test lint clean install uninstall init

build:
	@echo "==> building $(APP) for $$(go env GOOS)/$$(go env GOARCH)"
	@mkdir -p $(dir $(BUILD_OUTPUT))
	go build -o $(BUILD_OUTPUT) $(MAIN_PKG)
	@ls -la $(BUILD_OUTPUT)

test: lint
	@echo "==> running tests"
	go test ./internal/... -v

lint:
	@echo "==> linting"
	go vet ./...

clean:
	rm -rf ./bin

install: build
	ln -sfn $(CURDIR)/$(BUILD_OUTPUT) $(HOME)/.local/bin/$(APP)

uninstall:
	rm -f $(HOME)/.local/bin/$(APP)

init:
	@if [ ! -f config/localhost.yaml ]; then \
	  cp config/localhost.yaml.example config/localhost.yaml; \
	fi
	@if [ ! -f secrets/localhost.env ]; then \
	  cp secrets/env.example secrets/localhost.env; \
	fi
	@if [ ! -L .env ]; then \
	  ln -sf secrets/localhost.env .env; \
	fi
