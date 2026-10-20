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

CI uses Go 1.22 and golangci-lint 1.61.0.
