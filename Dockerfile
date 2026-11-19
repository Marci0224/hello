# Stage 1: build: has the Go toolchain
FROM golang:1.27 AS build
WORKDIR /src
# dependency files change rarely → download layer stays cached
COPY go.mod go.sum ./
RUN go mod download
# your code changes often → only this step and below rebuild
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /server .

# Stage 2: run: only what we copy across
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /server /server
# documentation: the server listens here (unless PORT says otherwise)
EXPOSE 8080
# numeric, so Kubernetes can verify it is not root
USER 65532:65532
ENTRYPOINT ["/server"]