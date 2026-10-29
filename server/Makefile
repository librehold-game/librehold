.PHONY: test e2e perf db db-down

test:
	go test ./...

# End-to-end tests against the built binary; build the web first (npm run build:web).
e2e:
	go test -tags e2e ./e2e/ -timeout 30m

# Performance tests (real time, about 15 minutes): run locally only, never in CI; build the web first.
perf:
	go test -tags 'e2e perf' ./e2e/ -run TestPerf -timeout 40m -v

db:
	docker compose -f docker-compose.dev.yml up -d

db-down:
	docker compose -f docker-compose.dev.yml down
