# Makefile for URL Shortener monorepo
#
# NOTE: Requires `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` to be
# installed and on $PATH. Install with:
#
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

PROTO_DIR=proto
GEN_DIR=proto/gen

.PHONY: proto migrate-up migrate-down

## Generate Go code from .proto definitions
proto:
	protoc \
		--go_out=. --go_opt=module=github.com/akuaruu/url-shortener \
		--go-grpc_out=. --go-grpc_opt=module=github.com/akuaruu/url-shortener \
		$(PROTO_DIR)/shortener.proto $(PROTO_DIR)/redirect.proto

## Apply database migrations (requires `migrate` CLI: https://github.com/golang-migrate/migrate)
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

## Roll back the last database migration
migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1
