# Contributing to Herald

## Development Setup

```bash
git clone https://github.com/kolapsis/herald.git
cd herald
make build
make test
```

## Running locally

```bash
cp configs/herald.example.yaml ~/.config/herald/herald.yaml
# Edit the config file
make run
```

## Testing

```bash
make test          # All tests with race detector
make test-cover    # Coverage report
make vet           # Go vet
make lint          # golangci-lint (install separately)
```

## Commit Conventions

We use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` — New feature
- `fix:` — Bug fix
- `docs:` — Documentation only
- `test:` — Adding or updating tests
- `refactor:` — Code change that neither fixes a bug nor adds a feature
- `chore:` — Maintenance tasks
- `ci:` — CI/CD changes

Scope is encouraged: `feat(mcp):`, `fix(auth):`, `docs(readme):`

## Branch Naming

- `feat/description` — Features
- `fix/description` — Bug fixes
- `docs/description` — Documentation

## Pull Requests

1. Fork the repo
2. Create your feature branch from `main`
3. Write tests for new functionality
4. Ensure `make lint`, `make test`, and `make vet` pass
5. Commit with conventional commit messages
6. Open a PR against `main`

## Code Style

- Go 1.26 standard formatting (`gofmt`)
- See `CLAUDE.md` for detailed conventions
