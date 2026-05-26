.PHONY: help lint test build ci install run lab lab-alloc lab-churn lab-idle lab-spike attach diff testbin release-snapshot

GOCMD ?= go
GCVIZ_RUN := $(GOCMD) run ./cmd/gcviz

PRESET ?= alloc
URL ?= http://127.0.0.1:8080/gcviz/metrics

help:
	@echo "Targets:"
	@echo "  make lint                Run golangci-lint"
	@echo "  make test                Run go tests"
	@echo "  make build               Build all packages (sanity check)"
	@echo "  make ci                  Lint + test + build"
	@echo "  make install             Install gcviz into GOPATH/bin"
	@echo ""
	@echo "Run modes (zero-guess):"
	@echo "  make lab                 Run lab preset (default PRESET=alloc)"
	@echo "  make lab-alloc            "
	@echo "  make lab-churn            "
	@echo "  make lab-idle             "
	@echo "  make lab-spike            "
	@echo "  make run TARGET=./app ARGS='-- --config ./cfg.yml'"
	@echo "  make attach               (default URL=$(URL))"
	@echo "  make attach URL=http://127.0.0.1:8080/gcviz/metrics"
	@echo "  make diff A=./a.json B=./b.json"
	@echo ""
	@echo "Maintainers:"
	@echo "  make testbin             Rebuild embedded testbin binaries"
	@echo "  make release-snapshot     Local goreleaser build (no publish)"

lint:
	golangci-lint run

test:
	$(GOCMD) test ./...

build:
	$(GOCMD) build ./...

ci: lint test build

install:
	$(GOCMD) install ./cmd/gcviz

lab:
	$(GCVIZ_RUN) lab $(PRESET)

lab-alloc:
	$(GCVIZ_RUN) lab alloc

lab-churn:
	$(GCVIZ_RUN) lab churn

lab-idle:
	$(GCVIZ_RUN) lab idle

lab-spike:
	$(GCVIZ_RUN) lab spike

run:
	$(GCVIZ_RUN) run $(TARGET) $(ARGS)

attach:
	$(GCVIZ_RUN) attach $(URL)

diff:
	$(GCVIZ_RUN) diff $(A) $(B)

testbin:
	$(GOCMD) run ./internal/devtools/testbinbuild

release-snapshot:
	goreleaser release --snapshot --clean
