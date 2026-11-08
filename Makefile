.PHONY: verify web-install web-build go-test dev db docker-build kind-deploy

verify: ; ./scripts/verify.sh
web-install: ; npm --prefix web ci
web-build: ; npm --prefix web run build
go-test: ; go test ./...

dev:
	docker compose up -d postgres
	TF_AUTH_DISABLED=true TF_DEV_USER_ROLE=devops TF_SEED=true go run ./server/cmd/server

db:
	docker compose up -d postgres

docker-build:
	docker build -t terraflow:local .

kind-deploy:
	bash scripts/kind-setup.sh
