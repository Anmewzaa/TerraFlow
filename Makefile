.PHONY: verify web-install web-build go-test

verify: ; ./scripts/verify.sh
web-install: ; npm --prefix web ci
web-build: ; npm --prefix web run build
go-test: ; go test ./...
