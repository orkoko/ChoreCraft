# ChoreCraft Backend

This directory contains the Go-based backend server for the ChoreCraft application.

## Prerequisites

*   Go (Golang) 1.22+
*   A running PostgreSQL database instance (see Database Setup).
*   [Docker Desktop](https://www.docker.com/products/docker-desktop/) (Optional, but recommended for database setup).

## 1. Database Setup

The application requires a running PostgreSQL database.

#### Option A: Local PostgreSQL Installation
If you have PostgreSQL installed and configured locally, ensure it is running and that you have a database created for this project.

#### Option B: Using Docker (Recommended)
If you don't have PostgreSQL installed or prefer using Docker, you can easily start a compatible database instance with a single command.

1.  Make sure Docker Desktop is running on your system.
2.  Open a terminal (PowerShell, Command Prompt, etc.) and run the following command:

    ```sh
    docker run --name chorecraft-db -e POSTGRES_USER=user -e POSTGRES_PASSWORD=password -e POSTGRES_DB=chorecraft -p 5432:5432 -d postgres
    ```
    This command downloads the official PostgreSQL image, starts a container named `chorecraft-db`, and automatically creates the `chorecraft` database with the `user` and `password` credentials. These values match the defaults in the `example.env` file.

You can verify the container is running via the Docker Desktop GUI or by running `docker ps` in your terminal.

## 2. Configuration

The server is configured using environment variables. An `example.env` file is provided as a template.

1.  **Create a `.env` file:**

    Copy the example configuration file to a new `.env` file. This file is ignored by Git, so it's safe to store your credentials here for local development.

    ```bash
    # From the C:/Users/orkok/IdeaProjects/ChoreCraft/backend directory
    cp example.env .env
    ```

2.  **Edit the `.env` file:**

    If you used a local PostgreSQL installation or different credentials with Docker, open the `.env` file and update the `DATABASE_URL` to match your setup. The default value already matches the Docker command provided above.

## 3. Database Schema and Migrations

The database schema is managed automatically by the application at startup using `golang-migrate`.

The raw SQL migration files are located in the `C:/Users/orkok/IdeaProjects/ChoreCraft/backend/migrations` directory. When the server starts, it will automatically apply any new migrations to your database, creating the necessary tables and columns. You no longer need to run the SQL scripts manually.

## 4. Running the Server

1.  **Navigate to the server's entry point:**

    ```bash
    cd C:/Users/orkok/IdeaProjects/ChoreCraft/backend/cmd/server
    ```

2.  **Install dependencies:**

    The project uses Go modules. Dependencies will be downloaded automatically when you build or run the project. You can also run `go mod tidy` from the `backend` directory to ensure all dependencies are present.

    ```bash
    # From C:/Users/orkok/IdeaProjects/ChoreCraft/backend
    go mod tidy
    ```

3.  **Run the server:**

    ```bash
    # From C:/Users/orkok/IdeaProjects/ChoreCraft/backend/cmd/server
    go run main.go
    ```

The server will now start, and you will see log messages indicating the status of the database migrations before the server begins listening for requests.
