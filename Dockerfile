FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.sum go.sum
COPY go.mod go.mod
RUN go mod download

COPY internal internal
COPY main.go main.go

RUN go vet
RUN go run golang.org/x/vuln/cmd/govulncheck@latest ./...
RUN go test ./...

RUN CGO_ENABLED=0 go build -o /src/app

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /src/app /
CMD ["/app"]
