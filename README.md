# EnvLens

Environment variable drift scanner. Finds inconsistencies between your code, `.env.example`, and `docker-compose.yml`.

## Install

```bash
go install github.com/envlens/envlens@latest
# or build from source
go build -o envlens .
```

## Usage

```bash
# Scan current directory
./envlens scan .

# CI mode (exits 1 on drift)
./envlens scan --ci .
```

## What It Detects

| Type | Description |
|------|-------------|
| **missing** | Var referenced in code but absent from `.env.example` |
| **ghost** | Var in `.env.example` but never referenced in code |
| **sensitive** | Var matching PASSWORD/SECRET/TOKEN/KEY exposed in `docker-compose.yml` |

## Supported Sources

- **Code**: Python (`os.getenv`, `os.environ`), Go (`os.Getenv`), TypeScript/JS (`process.env`), Java (`System.getenv`)
- **Config**: `.env`, `.env.example`, `.env.sample`
- **Infra**: `docker-compose.yml` / `docker-compose.yaml`

## GitHub Actions

```yaml
- name: EnvLens drift check
  run: |
    go install github.com/envlens/envlens@latest
    envlens scan --ci .
```

## License

MIT
