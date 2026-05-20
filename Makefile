.PHONY: build build-go run dev test fmt vet tidy clean frontend-dev frontend-build frontend-export embed-frontend release-build install

# Build dropboy with the latest UI bundle embedded.
build: embed-frontend
	cd backend && go build -o ../bin/dropboy ./cmd/dropboy

# Build without rebuilding the frontend — useful when iterating on Go code.
build-go:
	cd backend && go build -o ../bin/dropboy ./cmd/dropboy

run:
	cd backend && go run ./cmd/dropboy

# One-shot dev: starts the daemon (foreground) and the Next.js dev server
# together. Ctrl+C kills both. Open http://localhost:3000.
dev:
	@trap 'kill 0' INT TERM EXIT; \
	(cd backend && go run ./cmd/dropboy start --foreground) & \
	(cd frontend && npm run dev) & \
	wait

test:
	cd backend && go test ./...

fmt:
	cd backend && gofmt -s -w .

vet:
	cd backend && go vet ./...

tidy:
	cd backend && go mod tidy

frontend-dev:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build

# Static export (uses `output: 'export'` toggled by DROPBOY_EXPORT=1).
frontend-export:
	cd frontend && DROPBOY_EXPORT=1 npm run build

# Copy the Next.js export into the Go embed.FS directory.
embed-frontend:
	@rm -rf backend/internal/server/assets
	@mkdir -p backend/internal/server/assets
	@if [ -d frontend/out ]; then \
		cp -R frontend/out/. backend/internal/server/assets/; \
		echo "embedded frontend/out → backend/internal/server/assets"; \
	else \
		echo "frontend/out missing — run \`make frontend-export\` first; embedding placeholder"; \
		echo "Placeholder: run make frontend-export then make build." > backend/internal/server/assets/placeholder.txt; \
	fi

# Release-style local build: build frontend, embed, then build Go.
release-build: frontend-export embed-frontend build-go

# Install the built binary to /usr/local/bin (or ~/.local/bin if not writable).
install: build
	@target=/usr/local/bin/dropboy; \
	if [ ! -w /usr/local/bin ]; then target=$$HOME/.local/bin/dropboy; mkdir -p $$HOME/.local/bin; fi; \
	install -m 0755 bin/dropboy $$target; \
	echo "installed → $$target"

clean:
	rm -rf bin frontend/.next frontend/out frontend/node_modules backend/internal/server/assets
	@mkdir -p backend/internal/server/assets
	@echo "Placeholder file kept so //go:embed compiles." > backend/internal/server/assets/placeholder.txt
