# hello

A small Go server that counts visits in Redis, built to meet all twelve factors.

## Run it from a clean clone

### Locally (Go + Docker)

```sh
docker run -d --name redis -p 6379:6379 redis:8-alpine
go run .
curl localhost:8080/          # hello from my container (visit 1, served by <host>)
go run . reset                # counter back to zero
docker rm -f redis
```

### In Docker

```sh
docker build -t hello:0.2.0 .
docker run -d --name redis redis:8-alpine
docker run --rm -p 8080:8080 --link redis \
  -e REDIS_URL=redis://redis:6379/0 -e GREETING="hello from docker" hello:0.2.0
```

### In kind

```sh
docker build -t hello:0.2.0 .
kind load docker-image hello:0.2.0
kubectl apply -f k8s/
kubectl port-forward svc/hello 8080:80
curl localhost:8080/
# admin process, same image and config:
kubectl run reset --rm -it --restart=Never --image=hello:0.2.0 \
  --env=REDIS_URL=redis://redis:6379/0 -- reset
```

### Self-check

```sh
bash scripts/check-12factor.sh hello:0.2.0   # run from the course repository
```

## Configuration

| Variable    | Default                      | Meaning                          |
|-------------|------------------------------|----------------------------------|
| `PORT`      | `8080`                       | port to listen on                |
| `GREETING`  | `hello from my container`    | first part of the `/` answer     |
| `REDIS_URL` | `redis://localhost:6379/0`   | where the backing service is     |

## The twelve factors

| # | Factor | Where this repository meets it |
|---|--------|--------------------------------|
| I | Codebase | One repository, one branch (`hw1-twelve-factor`); the pull request is the whole change. |
| II | Dependencies | `go-redis` is declared in `go.mod` and pinned by `go.sum`; the Dockerfile runs `go mod download`, so nothing is assumed on the machine. |
| III | Config | `PORT`, `GREETING` and `REDIS_URL` are read from the environment in `main.go`; the cluster values are set in the `env` list of `k8s/deployment.yaml`. |
| IV | Backing services | Redis is reached only through `REDIS_URL` (`redis://redis:6379/0` in kind, `localhost` by default). While it is unreachable, `/` answers 503. |
| V | Build, release, run | One image, `hello:0.2.0`, built once; Docker and Kubernetes only differ in the environment they hand to it, with no rebuild. |
| VI | Processes | No state in memory: the counter lives in Redis (`INCR`), so both replicas share one rising count. |
| VII | Port binding | The server is its own HTTP server (`http.Server`) listening on `$PORT`; nothing is injected by a web server. |
| VIII | Concurrency | Scale out by running more processes: `replicas: 2` in `k8s/deployment.yaml` behind the one Service in `k8s/service.yaml`. |
| IX | Disposability | Starts in milliseconds; on SIGTERM `http.Server.Shutdown` finishes running requests (such as `/slow`) and the process exits 0. |
| X | Dev/prod parity | The same image runs in Docker and in kind, against the same `redis:8-alpine` image in both. |
| XI | Logs | `slog` JSON events on stdout, one per request (method, path, status, duration); go-redis' logger is redirected to slog, so nothing goes to stderr. The app never manages log files. |
| XII | Admin processes | `hello reset` zeroes the counter and exits; in the cluster it runs as a one-off pod from the same image and config. |
