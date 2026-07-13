.PHONY: dev dev-backend dev-frontend build clean

# Start backend in dev mode
dev-backend:
	cd backend && go run .

# Start frontend dev server
dev-frontend:
	cd frontend && npm run dev

# Start both frontend and backend (Ctrl+C kills both)
dev:
	@echo "Starting backend and frontend..."
	@trap 'kill 0' EXIT; \
	 (cd backend && go run .) & \
	 (cd frontend && npm run dev) & \
	 wait

# Build backend binary
build-backend:
	cd backend && go build -o server .

# Build frontend for production
build-frontend:
	cd frontend && npm run build
	rm -rf backend/static
	cp -r frontend/dist backend/static

# Full production build
build: build-backend build-frontend

# Clean build artifacts
clean:
	rm -f backend/server
	rm -rf backend/static
	rm -rf frontend/dist
	rm -f backend/data/runninghub.db
