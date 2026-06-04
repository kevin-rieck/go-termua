.PHONY: test run tidy test-integration

test:
	go test ./...

run:
	go run ./cmd/termua

tidy:
	go mod tidy

# Integration tests are expected to skip unless TERMUA_TEST_ENDPOINT is set.
test-integration:
	TERMUA_INTEGRATION=1 go test ./... -run Integration
