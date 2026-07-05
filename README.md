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

From the project root, copy the example configuration:
```bash
cp backend/example.env backend/.env
```

Open `backend/.env` and add your Gemini API key:
```env
GEMINI_API_KEY="your_key_here"
```

Then run the server from the project root:
```bash
go run backend/cmd/server/main.go
```

The API will be available at `http://localhost:8080`. Auto-generated Swagger docs are at `http://localhost:8080/swagger/index.html`.

> **Note:** The server automatically runs database schema setup on startup — no manual SQL needed.

---

### 3. Start the Frontend

In a new terminal, from the project root:
```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173` in your browser.

---

## 🌐 Remote Development & Tunnelling (ngrok)

To test the application remotely on mobile devices or share access:
1. Ensure the **Go Backend** is running on port `8080`.
2. Ensure the **Vite Frontend** is running on port `5173`.
3. In `frontend/vite.config.js`, make sure the proxy is configured to route `/api` to `http://localhost:8080` and `allowedHosts` is set to accept the tunnel domain:
   ```javascript
   allowedHosts: ['.ngrok-free.dev', '.ngrok-free.app']
   ```
4. Start the ngrok tunnel on the Vite port:
   ```bash
   ngrok http 5173
   ```
This maps both static assets and API requests (via Vite's built-in proxy) over a single public HTTPS URL.

---

## 🚢 Deployment

### 1. Build and Run Locally with Docker
You can run the backend server inside a Docker container:
```bash
# Navigate to backend directory
cd backend

# Build the Docker image
docker build -t chorecraft-backend .

# Run the container (pass your .env configurations)
docker run -p 8080:8080 --env-file .env chorecraft-backend
```

### 2. Deploy to Google Cloud Run (CI/CD)
The backend project includes a `cloudbuild.yaml` pipeline definition to automate deployment using **Google Cloud Build**:
```bash
cd backend
gcloud builds submit --config cloudbuild.yaml .
```
This triggers a Google Cloud Build runner to:
1. Compile the Go binary and package it into a container using the `Dockerfile`.
2. Push the built container image to your Google Container Registry (`gcr.io`).
3. Deploy/update the containerized service automatically to **Google Cloud Run** in `us-central1` with unauthenticated access enabled.

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
