# tool versions
golangci_lint_version := "v1.54.0"

# test and lint the code
default: test lint

# run isolated tests
test:
    go test ./... -timeout 10s -race -shuffle on

# lint the whole project
lint:
    go run github.com/golangci/golangci-lint/cmd/golangci-lint@{{golangci_lint_version}} run ./...

# format Go code
fmt:
    go fmt ./...

# run vulncheck
vulncheck:
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...
