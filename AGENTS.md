# Agent notes

## Build

```bash
go build ./...
```

## Test

```bash
go test -race ./...
```

## Lint

```bash
go vet ./...
golangci-lint run
```

CI uses Go 1.22 and golangci-lint 2.15.0.
