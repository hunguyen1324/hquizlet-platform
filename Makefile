.PHONY: dev-infra test-go test-rust

dev-infra:
	docker compose -f infra/docker/docker-compose.yml up -d

test-go:
	go test ./...

test-rust:
	cargo test --workspace

