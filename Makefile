.PHONY: dev-infra test-go test-rust test-golden test-phase4 test-phase9 lint-openapi test-frontend test-e2e test-all ci-gate e2e-phase9

# ── Infrastructure ──────────────────────────────────────────────────────────────
dev-infra:
	docker compose -f infra/docker/docker-compose.yml up -d

# ── Go ──────────────────────────────────────────────────────────────────────────
test-go:
	cd services/quiz && go test ./... -count=1
	cd services/file && go test ./... -count=1

build-go:
	cd services/quiz && go build ./...
	cd services/file && go build ./...

# ── Rust ────────────────────────────────────────────────────────────────────────
test-rust:
	cargo test --workspace

clippy-rust:
	cargo clippy --workspace --all-targets -- -D warnings

# ── Golden vectors ──────────────────────────────────────────────────────────────
test-golden:
	cd crates/quiz-core && cargo test golden -- --nocapture
	cd services/quiz && go test -v -run Golden -count=1
	cd packages/api-contracts/examples/quiz && node validate-examples.js

# ── Frontend ────────────────────────────────────────────────────────────────────
test-frontend:
	cd apps/web && npx vitest run

build-frontend:
	cd apps/web && npm run build

# ── Contract lint ───────────────────────────────────────────────────────────────
lint-openapi:
	npx --yes @redocly/cli@latest lint packages/api-contracts/openapi.yaml

# ── E2E ─────────────────────────────────────────────────────────────────────────
test-e2e:
	bash infra/scripts/phase4-e2e.sh

# ── Phase 9 gate ──────────────────────────────────────────────────────────────
test-phase9: test-go test-frontend lint-openapi
	@echo "Phase 9 gate OK"

e2e-phase9:
	bash infra/scripts/phase9-e2e.sh

# ── Combined phase4 gate ───────────────────────────────────────────────────────
test-phase4: test-go test-rust test-golden lint-openapi test-frontend

# ── Full CI gate (everything that must pass) ────────────────────────────────────
ci-gate: lint-openapi test-golden test-frontend test-rust test-go
	@echo ""
	@echo "═══════════════════════════════════════════════════════════"
	@echo "  CI GATE: ALL CHECKS PASSED"
	@echo "═══════════════════════════════════════════════════════════"
