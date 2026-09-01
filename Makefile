.PHONY: dev-infra test-go test-rust test-golden test-phase4 lint-openapi

dev-infra:
	docker compose -f infra/docker/docker-compose.yml up -d

test-go:
	go test ./...

test-rust:
	cargo test --workspace

test-golden:
	cargo test --package quiz-core -- golden
	node packages/api-contracts/examples/quiz/validate-examples.js

test-phase4: test-go test-rust test-golden
	npx --yes @redocly/cli@latest lint packages/api-contracts/openapi.yaml

lint-openapi:
	npx --yes @redocly/cli@latest lint packages/api-contracts/openapi.yaml

