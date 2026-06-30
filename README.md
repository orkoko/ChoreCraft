# 🎯 ChoreCraft

> A gamified household chore management platform powered by AI — making chores fun for kids and easy to manage for parents.

ChoreCraft is a full-stack web application that helps families organise household chores through a gamified reward system. Kids earn coins by completing chores, parents approve submissions, and the whole family can collaborate on group tasks and shared rewards.

Task icons are automatically assigned by **Google Gemini AI**, which analyses the task name and picks a fitting emoji — giving each chore a unique visual identity the moment it's created.

---

## ✨ Features

- **Role-based interface** — Separate parent and kid views with appropriate permissions
- **AI icon mapping** — Google Gemini automatically assigns relevant emojis to tasks and rewards
- **Mandatory task enforcement** — Non-optional chores block optional ones until fully approved by a parent
- **Cooperative tasks** — Group tasks that require all household members to contribute to a shared pool
- **Reward shop** — Kids spend earned coins on rewards; parents approve purchases
- **Real-time sync** — All connected browsers update instantly via Server-Sent Events (SSE)
- **Point system** — Transparent leaderboard and statistics per chore group

---

## 🗂️ Project Structure

```
ChoreCraft/
├── backend/        # Go REST API server
│   ├── cmd/server/ # Application entry point & integration tests
│   ├── internal/
│   │   ├── config/     # Environment variable loading
│   │   ├── handler/    # HTTP route handlers
│   │   ├── model/      # Shared data structures
│   │   ├── repository/ # PostgreSQL data access layer
│   │   └── service/    # Business logic & Gemini AI integration
│   ├── example.env     # Configuration template
│   └── README.md       # Backend setup guide
│
└── frontend/       # Vanilla JS single-page application
    ├── index.html  # App shell
    ├── app.js      # All UI logic and API calls
    ├── styles.css  # Full design system
    ├── icon.svg    # Application icon
    └── README.md   # Frontend setup guide
```

---

## 🛠️ Tech Stack

| Layer     | Technology                              |
|-----------|-----------------------------------------|
| Backend   | Go 1.22+, chi router, pgx (PostgreSQL)  |
| Database  | PostgreSQL 16                           |
| AI        | Google Gemini API (gemini-flash-latest) |
| Frontend  | Vanilla HTML, CSS, JavaScript           |
| Dev Server | Vite 5                                 |

---

## 🚀 Quick Start

### Prerequisites

- [Go](https://golang.org/dl/) 1.22+
- [Node.js](https://nodejs.org/) v18+
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (recommended for database)
- A [Google Gemini API Key](https://aistudio.google.com/app/apikey) (free tier available)

---

### 1. Start the Database

```bash
docker run --name chorecraft-db \
  -e POSTGRES_USER=user \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=chorecraft \
  -p 5432:5432 -d postgres
```

---

### 2. Configure & Start the Backend

```bash
cd backend

# Copy the example configuration
cp example.env .env
```

Open `backend/.env` and add your Gemini API key:
```env
GEMINI_API_KEY="your_key_here"
```

Then run the server:
```bash
go run cmd/server/main.go
```

The API will be available at `http://localhost:8080`. Auto-generated Swagger docs are at `http://localhost:8080/swagger/index.html`.

> **Note:** The server automatically runs database schema setup on startup — no manual SQL needed.

---

### 3. Start the Frontend

```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173` in your browser.

---

## 🧪 Testing

The backend includes integration tests that use Docker to spin up a real PostgreSQL database via [Testcontainers](https://golang.testcontainers.org/).

```bash
# Run all backend tests (requires Docker)
cd backend
go test ./...

# Run only the Gemini AI integration test (requires a valid GEMINI_API_KEY in .env)
go test -v ./cmd/server -run TestGeminiIntegration
```

---

## ⚙️ Configuration Reference

All backend configuration is done via environment variables. See [`backend/example.env`](./backend/example.env) for the full template.

| Variable       | Required | Default                                                    | Description                            |
|----------------|----------|------------------------------------------------------------|----------------------------------------|
| `DATABASE_URL` | Yes      | `postgres://user:password@localhost:5432/chorecraft?sslmode=disable` | PostgreSQL connection string |
| `PORT`         | No       | `8080`                                                     | HTTP server port                       |
| `GEMINI_API_KEY` | No     | _(empty)_                                                  | Google Gemini API key for AI icons     |

> If `GEMINI_API_KEY` is not set, tasks will still be created successfully — they just won't receive an AI-generated emoji icon.

---

## 📖 Further Reading

- [Backend README](./backend/README.md) — Detailed backend setup, schema info, and running instructions
- [Frontend README](./frontend/README.md) — Frontend architecture, scripts, and project structure
