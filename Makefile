.PHONY: start

start:
	@docker-compose up --build

test:
	@go test -v -p 1 ./...
