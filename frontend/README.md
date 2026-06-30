# ChoreCraft Frontend

A vanilla HTML/CSS/JavaScript single-page application (SPA) that provides the user interface for the ChoreCraft gamified chore management platform. Built with [Vite](https://vitejs.dev/) as the development server.

## Overview

The frontend offers two distinct role-based views:

- **Parent/Admin View** — Create and manage chore groups, tasks, and rewards. Approve or reject chore submissions and reward purchase requests.
- **Kid/User View** — View assigned chores, submit completed chores for approval, track earned points, and redeem rewards.

Key features include:
- 🤖 **AI-powered task icons** — Emojis are automatically assigned to tasks via the Gemini API on the backend.
- 🔄 **Real-time updates** — The UI uses Server-Sent Events (SSE) to instantly reflect changes across all connected tabs.
- 🏆 **Gamified point system** — Kids earn coins for completed chores and can spend them in a shared reward shop.
- 🚦 **Mandatory task enforcement** — Non-mandatory chores are automatically grayed out until all mandatory tasks are fully approved.

## Tech Stack

| Layer      | Technology                         |
|------------|------------------------------------|
| Structure  | HTML5 (semantic)                   |
| Styling    | Vanilla CSS (custom design system) |
| Logic      | Vanilla JavaScript (ES Modules)    |
| Dev Server | Vite 5                             |
| Fonts      | Google Fonts (Nunito)              |

## Prerequisites

- [Node.js](https://nodejs.org/) v18 or higher (for Vite)
- The [ChoreCraft Backend](../backend/README.md) must be running at `http://localhost:8080`

## Getting Started

### 1. Install dependencies

```bash
# From the frontend/ directory
npm install
```

### 2. Start the development server

```bash
npm run dev
```

Vite will start a development server accessible at:
```
http://localhost:5173
```

The app will automatically proxy API calls to the backend at `http://localhost:8080/api`.

## Project Structure

```
frontend/
├── index.html      # Main HTML shell — all views are rendered here
├── app.js          # All application logic, state management, and API calls
├── styles.css      # Full design system: variables, components, layouts
├── icon.svg        # Application icon (favicon + nav logo)
├── manifest.json   # PWA web app manifest
└── package.json    # Vite dev dependency
```

## API

The frontend communicates with the backend REST API. All requests are prefixed with `/api`. The full Swagger/OpenAPI spec is available from the backend at:

```
http://localhost:8080/swagger/index.html
```

## Available Scripts

| Command         | Description                              |
|-----------------|------------------------------------------|
| `npm run dev`   | Start Vite dev server with hot reload    |
| `npm run build` | Build production bundle to `dist/`       |
| `npm run preview` | Preview the production build locally   |
