.PHONY: test e2e db db-down

test:
	go test ./...

# End-to-end tests against the built binary; build the web first (npm run build:web).
e2e:
	go test -tags e2e ./e2e/ -timeout 30m

db:
	docker compose -f docker-compose.dev.yml up -d

db-down:
	docker compose -f docker-compose.dev.yml down
