.PHONY: lint fmt ci test devdeps
LINTER := golangci-lint
build:
	go build -o bin/yas3 .
ci: devdeps lint test
run:
	go run . --config example/yas3.toml

lint:
	@echo ">> Running linter ($(LINTER))"
	$(LINTER) run

fmt:
	@echo ">> Formatting code"
	gofmt -w .
	goimports -w .

test:
	@echo ">> Running tests"
	go test -v -cover ./...

devdeps:
	@echo ">> Installing development dependencies"
	which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	which mockgen > /dev/null || go install go.uber.org/mock/mockgen@latest

up:
	docker compose up -d

down:
	docker compose down
	docker compose rm -f

clean:
	rm -rf docker/dynamodb

reset: down clean up
