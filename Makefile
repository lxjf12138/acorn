GO_MODULES := \
	./packages/api \
	./packages/servicekit \
	./services/agent-control-plane \
	./services/sandbox-service \
	./packages/core \
	./packages/mcp \
	./packages/signal \
	./packages/provider-sdk-go

SERVICE_MODULES := \
	./services/agent-control-plane \
	./services/sandbox-service

BIN_DIR := ./bin

.PHONY: fmt fmt-check vet test build proto-lint proto-gen proto-check ci

fmt:
	@set -e; \
	for module in $(GO_MODULES); do \
		echo "==> gofmt $$module"; \
		files=$$(find $$module -type f -name '*.go'); \
		if [ -n "$$files" ]; then \
			gofmt -w $$files; \
		fi; \
	done

fmt-check:
	@set -e; \
	for module in $(GO_MODULES); do \
		files=$$(find $$module -type f -name '*.go'); \
		if [ -n "$$files" ]; then \
			out=$$(gofmt -l $$files); \
			if [ -n "$$out" ]; then \
				echo "$$out"; \
				exit 1; \
			fi; \
		fi; \
	done

vet:
	@set -e; \
	for module in $(GO_MODULES); do \
		echo "==> go vet $$module"; \
		(cd $$module && go vet ./...); \
	done

test:
	@set -e; \
	for module in $(GO_MODULES); do \
		echo "==> go test $$module"; \
		(cd $$module && go test ./...); \
	done

build:
	@set -e; \
	mkdir -p $(BIN_DIR); \
	for module in $(SERVICE_MODULES); do \
		name=$$(basename $$module); \
		echo "==> go build $$name"; \
		(cd $$module && go build -o ../../bin/$$name ./cmd/$$name); \
	done

proto-lint:
	@buf lint

proto-gen:
	@buf generate

proto-check:
	@buf generate
	@test -z "$$(git diff --name-only -- packages/api/gen)" || \
		(git diff --name-only -- packages/api/gen; exit 1)
	@test -z "$$(git ls-files --others --exclude-standard -- packages/api/gen)" || \
		(git ls-files --others --exclude-standard -- packages/api/gen; exit 1)

ci: proto-lint proto-check fmt-check vet test build
