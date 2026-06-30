# ChoreCraft Backend

The Go-based REST API server for the ChoreCraft application. It handles user auth, chore group management, task and reward lifecycle, cooperative purchases, real-time SSE events, and AI-powered emoji icon mapping via Google Gemini.

## Prerequisites

- [Go](https://golang.org/dl/) 1.22+
- A running PostgreSQL 16 database instance (see Database Setup)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (optional, but recommended)
- A [Google Gemini API Key](https://aistudio.google.com/app/apikey) (optional, enables AI emoji icons)

## 1. Database Setup

#### Option A: Using Docker (Recommended)

```bash
docker run --name chorecraft-db \
  -e POSTGRES_USER=user \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=chorecraft \
  -p 5432:5432 -d postgres
```

These credentials match the defaults in `example.env`.

#### Option B: Local PostgreSQL

Ensure PostgreSQL is running and create a database. Update `DATABASE_URL` in your `.env` accordingly.

## 2. Configuration

1. **Copy the example config:**

    ```bash
    # From the backend/ directory
    cp example.env .env
    ```

2. **Edit `.env`** and fill in your values. At minimum, set `GEMINI_API_KEY` if you want AI emoji icons:

    ```env
    DATABASE_URL="postgres://user:password@localhost:5432/chorecraft?sslmode=disable"
    PORT="8080"
    GEMINI_API_KEY="your_gemini_api_key_here"
    ```

## 3. Database Schema

The schema is applied **automatically on server startup** using an embedded SQL file. No manual migration steps are required.

## 4. Running the Server

```bash
# From the backend/ directory
go run cmd/server/main.go
```

The server starts on port `8080` (or `PORT` from your `.env`). Swagger API docs are available at:

```
http://localhost:8080/swagger/index.html
```

## 5. Running Tests

Tests use [Testcontainers](https://golang.testcontainers.org/) to spin up a real PostgreSQL database in Docker automatically.

```bash
# From the backend/ directory — runs all tests
go test ./...

# Run only the Gemini AI integration test (requires a valid GEMINI_API_KEY in .env)
go test -v ./cmd/server -run TestGeminiIntegration
```

## Project Structure

```
backend/
├── cmd/server/         # main.go entry point + integration tests
├── internal/
│   ├── config/         # Environment variable loading (godotenv)
│   ├── handler/        # HTTP handlers (chi router)
│   ├── model/          # Shared request/response data types
│   ├── repository/     # PostgreSQL queries (pgx)
│   └── service/        # Business logic + Gemini AI integration
├── example.env         # Configuration template
├── go.mod
└── go.sum
```
