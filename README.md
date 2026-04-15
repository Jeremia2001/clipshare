How to Run:
1. Start the Infrastructure (Docker Compose):
cd clipshare/docker
docker-compose up -d
This starts:
- PostgreSQL on port 5432
- Redis on port 6379  
- MinIO (S3-compatible) on ports 9000/9001
- Go API server on port 8080
2. Run Migrations:
cd clipshare/apps/server
go run cmd/migrate/main.go up
3. Run the Server (for development):
cd clipshare/apps/server
go run cmd/api/main.go
4. Build the Desktop App (with webkit2_41 tag for Ubuntu 24.04):
cd clipshare/apps/desktop
npm install --prefix frontend
wails build -tags webkit2_41
# Or for dev mode:
wails dev -tags webkit2_41