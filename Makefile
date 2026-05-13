GO_MODULES := \
	./services/agent-control-plane \
	./services/capability-node \
	./packages/core \
	./packages/mcp \
	./packages/signal \
	./packages/provider-sdk-go

SERVICE_MODULES := \
	./services/agent-control-plane \
	./services/capability-node

BIN_DIR := ./bin

.PHONY: fmt fmt-check vet test build ci

fmt:
	@for module in $(GO_MODULES); do \
		echo "==> gofmt $$module"; \
		files=$$(find $$module -type f -name '*.go'); \
		if [ -n "$$files" ]; then \
			gofmt -w $$files; \
		fi; \
	done

fmt-check:
	@for module in $(GO_MODULES); do \
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
	@for module in $(GO_MODULES); do \
		echo "==> go vet $$module"; \
		(cd $$module && go vet ./...); \
	done

test:
	@for module in $(GO_MODULES); do \
		echo "==> go test $$module"; \
		(cd $$module && go test ./...); \
	done

build:
	@mkdir -p $(BIN_DIR)
	@for module in $(SERVICE_MODULES); do \
		name=$$(basename $$module); \
		echo "==> go build $$name"; \
		(cd $$module && go build -o ../../bin/$$name ./cmd/$$name); \
	done

ci: fmt-check vet test build
