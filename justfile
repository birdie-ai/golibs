# test and lint the code
default: test lint

covreport := "coverage.txt"

# run tests
test:
    go test ./... -timeout 10s -race -shuffle on

# run tests and generate coverage report
test-coverage:
	go test -count=1 -coverprofile={{covreport}} ./...

# run tests with coverage report and show the report on the local browser
test-coverage-show: test-coverage
	go tool cover -html={{covreport}}

# lint the whole project
lint:
    golangci-lint run ./...

# format Go code
fmt:
    go fmt ./...

# run vulncheck
vulncheck:
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...
