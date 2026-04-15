# Contributing to Defuddle

Thanks for your interest in contributing! This guide covers the basics.

## Development Setup

- **Go 1.26+** required
- Clone the repo and initialize the reference library submodule:

```bash
git clone <repo-url> && cd defuddle
make submodules
```

## Commands

| Command | Description |
|---------|-------------|
| `task dev` | Quick dev cycle (format, vet, test) |
| `task verify` | Full verification (fmt, vet, lint, test) |
| `task test` | Run all tests |
| `task lint` | Run linters |
| `task build-cli` | Build the CLI binary |

## Pull Requests

- Run `task verify` before submitting — all checks must pass
- Keep changes focused on a single concern
- Follow the existing code style and patterns
- Maintain API compatibility with the original TypeScript Defuddle library — same inputs should produce the same outputs

## License

By contributing, you agree that your code will be licensed under the MIT License.
