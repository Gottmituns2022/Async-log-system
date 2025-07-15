FROM golang:1.23
WORKDIR /app
COPY . .
RUN go mod download
CMD ["go", "test", "-v", "-race"]