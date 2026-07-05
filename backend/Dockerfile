# Start with a base image
FROM golang:1.21-alpine

# Set the working directory
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download the dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the application
RUN go build -o /main cmd/main.go

# Expose the port the application will run on
EXPOSE 8080

# Run the application
CMD ["/main"]
