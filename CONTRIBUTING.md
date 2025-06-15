# Contributing to Mutating Registry Webhook

Thank you for your interest in contributing to this project! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please note that this project is released with a Contributor Code of Conduct. By participating in this project you agree to abide by its terms.

## How to Contribute

### Reporting Issues

Before creating an issue, please check if it already exists. When creating an issue, provide:

- Clear description of the issue
- Steps to reproduce
- Expected behavior
- Actual behavior
- Environment details (Kubernetes version, platform, etc.)

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add or update tests as needed
5. Update documentation if needed
6. Run tests (`make test`)
7. Commit your changes (`git commit -m 'Add amazing feature'`)
8. Push to the branch (`git push origin feature/amazing-feature`)
9. Open a Pull Request

### Development Setup

1. **Prerequisites**:
   - Go 1.22+
   - Docker
   - kubectl
   - kubebuilder (optional, for API changes)

2. **Clone the repository**:
   ```bash
   git clone https://github.com/flemzord/mutating-registry-webhook.git
   cd mutating-registry-webhook
   ```

3. **Install dependencies**:
   ```bash
   go mod download
   ```

4. **Run tests**:
   ```bash
   make test
   ```

5. **Build**:
   ```bash
   make build
   ```

### Testing

- **Unit tests**: `make test`
- **Integration tests**: `make test-integration`
- **E2E tests**: `make test-e2e` (requires Kind)

### Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Run `golangci-lint run` to check for issues
- Keep functions small and focused
- Write clear, descriptive comments

### Commit Messages

Follow the conventional commits specification:

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `test:` Test additions or fixes
- `refactor:` Code refactoring
- `chore:` Maintenance tasks

Example: `feat: add support for ghcr.io registry`

### Documentation

- Update README.md for user-facing changes
- Add godoc comments for exported functions
- Update Helm chart values documentation

## Release Process

Releases are automated via GitHub Actions when a tag is pushed:

```bash
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

This will:
- Create a GitHub release
- Build and push multi-arch Docker images
- Package and push Helm chart
- Generate installation manifests

## Questions?

Feel free to open an issue for any questions about contributing!