GO ?= go
GOBIN ?= $$($(GO) env GOPATH)/bin
GOLANGCI_LINT ?= $(GOBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.0.1 # LINT_VERSION: update version in other places
TEST_COVERAGE ?= $(GOBIN)/go-test-coverage

.PHONY: install-golangcilint
install-golangcilint:
	test -f $(GOLANGCI_LINT) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

# Runs lint on entire repo
.PHONY: lint
lint: install-golangcilint
	$(GOLANGCI_LINT) run ./...

# Runs benchmark on entire repo
.PHONY: benchmark
benchmark:
	go version
	go test -bench=. github.com/vladopajic/go-actor/actor -run=^# -count 5  -benchmem

# Runs tests on entire repo
.PHONY: test
test: 
	go test -timeout=10s -race -count=10 -shuffle=on -failfast ./...

# Runs experimental tests
.PHONY: test-experimental
test-experimental:
	go mod edit -go 1.24
	go env -w GOEXPERIMENT=synctest
	go test -tags "experimental" -run Experimental$$ -timeout=10s -race -count=10 -shuffle=on -failfast -v ./...
	go env -u GOEXPERIMENT
	go mod edit -go 1.22

# Code tidy
.PHONY: tidy
tidy:
	go mod tidy
	go fmt ./...

.PHONY: install-go-test-coverage
install-go-test-coverage:
	go install github.com/vladopajic/go-test-coverage/v2@latest


# Generates test coverage profile
.PHONY: generate-coverage
generate-coverage:
	go test ./... -coverprofile=./cover.out -covermode=atomic -coverpkg=./...

# Runs test coverage check
.PHONY: check-coverage
check-coverage: generate-coverage
check-coverage: install-go-test-coverage
	$(TEST_COVERAGE) -config=./.testcoverage.yml

# View coverage profile
.PHONY: view-coverage
view-coverage: generate-coverage
	go tool cover -html=cover.out -o=cover.html
	xdg-open cover.html