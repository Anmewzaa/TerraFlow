.DEFAULT_GOAL := help
.PHONY: help verify ci fmt vet go-test test-pg fuzz web-install web-lint web-test web-build web-dev openapi-check build run dev db db-down docker-build kind-deploy clean

# The database the Postgres-backed tests may create databases in (see docs/06-conventions.md).
TF_TEST_PG_DSN ?= postgres://terraflow:localdev@localhost:5432/terraflow?sslmode=disable
export TF_TEST_PG_DSN

help: ## List the targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-15s %s\n", $$1, $$2}'

verify: ## Everything, as scripts/verify.sh runs it
	./scripts/verify.sh

ci: fmt vet test-pg web-lint web-test web-build openapi-check ## What a pull request must pass

# --- Go ---------------------------------------------------------------------
fmt: ## Fail when a Go file is not gofmt-ed
	@out="$$(gofmt -l server)"; if [ -n "$$out" ]; then echo "gofmt:"; echo "$$out"; exit 1; fi

vet: ## go vet
	go vet ./...

go-test: ## Go tests (Postgres tests are skipped without TF_TEST_PG_DSN)
	go test ./... -count=1

test-pg: db ## Go tests including the Postgres-backed ones (starts the compose database)
	go test ./... -count=1

fuzz: ## A short fuzz run of the canvas value checker
	go test ./server/internal/validate -run xxx -fuzz FuzzValuesNeverPanic -fuzztime 15s

build: ## Build the server binary into bin/
	go build -o bin/terraflow ./server/cmd/server

run: ## Run the server with the settings in .env.local
	go run ./server/cmd/server

# --- Web --------------------------------------------------------------------
web-install: ## npm ci
	npm --prefix web ci

web-lint: ## eslint and the TypeScript check
	npm --prefix web run lint
	npx --prefix web tsc --noEmit -p web

web-test: ## vitest
	npm --prefix web test -- --run

web-build: ## Production build of the web app
	npm --prefix web run build

web-dev: ## The Vite dev server (the API is expected on :8080)
	npm --prefix web run dev

# --- Contract ---------------------------------------------------------------
openapi-check: ## api/openapi.yaml must be valid YAML with its paths
	node -e "const y=require('./web/node_modules/js-yaml');const d=y.load(require('fs').readFileSync('api/openapi.yaml','utf8'));if(!d.paths||!Object.keys(d.paths).length)process.exit(1);console.log('openapi ok:',Object.keys(d.paths).length,'paths')"

# --- Local environment ------------------------------------------------------
dev: db ## Database, then the server with made-up sign-in users
	TF_AUTH_DISABLED=true TF_DEV_LOGIN=true TF_DEV_USER_ROLE=devops TF_SEED=true go run ./server/cmd/server

db: ## Start the local Postgres
	docker compose up -d postgres

db-down: ## Stop the local Postgres
	docker compose stop postgres

docker-build: ## Build the image as terraflow:local
	docker build -t terraflow:local .

kind-deploy: ## Deploy to a local kind cluster
	bash scripts/kind-setup.sh

clean: ## Remove build output
	rm -rf bin web/dist
