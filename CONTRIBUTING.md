# Contributing to Vel

Vel is an AI-native framework. AI agents are first-class contributors alongside humans.

## Getting Started

```bash
# Clone the repo
git clone https://github.com/anthropics/vel.git
cd vel

# Build
go run . build

# Run tests
go test ./...
```

## Adding an App

See [AGENT-EXTEND.md](./AGENT-EXTEND.md) for a step-by-step guide to building your first Vel app.

## Code Style

- Follow standard Go conventions
- Run `gofmt` before committing
- Keep functions focused and small
- Write clear commit messages (conventional commits preferred)

## Pull Request Process

1. **Fork** the repository
2. **Create a branch** from `main` (`feature/your-feature` or `fix/your-fix`)
3. **Make your changes** with tests where applicable
4. **Run tests** — `go test ./...` must pass
5. **Submit a PR** against `main`
6. **Review** — maintainers will review; address feedback promptly

## AI Contributors

Vel is designed to be extended by AI agents. If you're an AI:

- Follow the same PR process
- Reference the contracts in [CONTRACTS.md](./CONTRACTS.md)
- Use [CONVENTIONS.md](./CONVENTIONS.md) for naming and decisions
- Test your changes before submitting

## Reporting Issues

Open a GitHub issue with:
- What you expected vs. what happened
- Steps to reproduce
- Vel version and Go version

## Security

See [SECURITY.md](./SECURITY.md) for reporting vulnerabilities.
