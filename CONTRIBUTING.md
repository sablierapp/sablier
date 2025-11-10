# Contributing to Sablier

Thank you for your interest in contributing to Sablier! This guide will help you get started with contributing to the project.

## Table of Contents

- [Contributing to Sablier](#contributing-to-sablier)
  - [Table of Contents](#table-of-contents)
  - [Getting Started](#getting-started)
    - [Fork the Repository](#fork-the-repository)
    - [Clone Your Fork](#clone-your-fork)
    - [Create a Branch](#create-a-branch)
  - [Contribution Guidelines](#contribution-guidelines)
    - [Conventional Commits](#conventional-commits)
  - [Development Guide](#development-guide)
    - [Compiling](#compiling)
      - [Build the Binary](#build-the-binary)
      - [Build for Specific Platform](#build-for-specific-platform)
      - [Run Locally](#run-locally)
      - [Build Docker Image](#build-docker-image)
    - [Testing](#testing)
      - [Run All Tests](#run-all-tests)
      - [Run Linters](#run-linters)
    - [Running Long Tests (Testcontainers)](#running-long-tests-testcontainers)
      - [Running Testcontainer Tests](#running-testcontainer-tests)
      - [Skipping Long Tests](#skipping-long-tests)
  - [Submitting Your Contribution](#submitting-your-contribution)
    - [Before Submitting](#before-submitting)
    - [Create a Pull Request](#create-a-pull-request)
    - [After Merge](#after-merge)
  - [Getting Help](#getting-help)
  - [Code of Conduct](#code-of-conduct)
  - [License](#license)

---

## Getting Started

### Fork the Repository

1. Navigate to the [Sablier repository](https://github.com/sablierapp/sablier)
2. Click the **Fork** button in the top-right corner
3. This creates a copy of the repository under your GitHub account

### Clone Your Fork

```bash
git clone https://github.com/YOUR_USERNAME/sablier.git
cd sablier
```

### Create a Branch

Create a new branch for your feature or bug fix:

```bash
git checkout -b my-new-feature
```

## Contribution Guidelines

### Conventional Commits

We use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages. This leads to more readable messages and helps with automated versioning and changelog generation.

**Format:**
```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, missing semicolons, etc.)
- `refactor`: Code refactoring without changing functionality
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `build`: Changes to build system or dependencies
- `ci`: Changes to CI configuration
- `chore`: Other changes that don't modify src or test files

**Examples:**
```bash
git commit -m "feat(provider): add Podman support"
git commit -m "fix(api): resolve race condition in session management"
git commit -m "docs: update installation guide with new Docker image"
git commit -m "test(kubernetes): add integration tests for StatefulSets"
```

**Breaking Changes:**

For breaking changes, add `!` after the type or add `BREAKING CHANGE:` in the footer:

```bash
git commit -m "feat(api)!: remove deprecated /v1/start endpoint"

# or

git commit -m "feat(api): update authentication mechanism

BREAKING CHANGE: API now requires API key in header instead of query parameter"
```

---

## Development Guide

### Compiling

#### Build the Binary

```bash
make build
```

This creates a binary in the current directory (e.g., `sablier_draft_linux-amd64`).

#### Build for Specific Platform

```bash
# Linux
GOOS=linux GOARCH=amd64 make build

# macOS
GOOS=darwin GOARCH=amd64 make build

# Windows
GOOS=windows GOARCH=amd64 make build
```

#### Run Locally

```bash
# Build and run
make build
./sablier_draft_linux-amd64 start --help

# Or run directly
go run cmd/sablier/sablier.go start --provider.name=docker
```

#### Build Docker Image

```bash
docker build -t sablier:dev .
```

---

### Testing

#### Run All Tests

```bash
make test
```

Or using Go directly:

```bash
go test ./...
```

#### Run Linters

```bash
make lint
```

---

### Running Long Tests (Testcontainers)

Some tests use [Testcontainers](https://golang.testcontainers.org/) to spin up real containers (Docker, Kubernetes, etc.). These tests are longer-running and require Docker to be installed and running.

#### Running Testcontainer Tests

By default, testcontainer tests are included in the regular test suite:

```bash
make test
```

#### Skipping Long Tests

If you want to skip testcontainer tests during development:

```bash
go test -short ./...
```

Mark long-running tests with:

```go
func TestMyLongRunningTest(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping long-running test")
    }
    // Test code...
}
```

## Submitting Your Contribution

### Before Submitting

```bash
make fmt
make test
make lint
```


### Create a Pull Request

1. **Push your branch**:
   ```bash
   git push origin my-new-feature
   ```

2. **Open a Pull Request** on GitHub:
   - Go to your fork on GitHub
   - Click "Compare & pull request"
   - Fill in the PR template with:
     - Description of changes
     - Related issues (if any)
     - Testing done
     - Screenshots (if UI changes)

3. **PR Title**: Follow conventional commit format:
   ```
   feat(provider): add Podman support
   fix(api): resolve session race condition
   ```

4. **Address review comments**:
   - Make requested changes
   - Push additional commits
   - Re-request review when ready

### After Merge

Your changes will be included in the next release. Thank you for contributing! 🎉

---

## Getting Help

- **Discord**: Join our [Discord server](https://discord.gg/WXYp59KeK9) for discussions and support
- **Issues**: Check [existing issues](https://github.com/sablierapp/sablier/issues) or create a new one
- **Documentation**: Read the [full documentation](https://sablierapp.dev)

---

## Code of Conduct

Please note that this project is released with a [Code of Conduct](CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.

---

## License

By contributing to Sablier, you agree that your contributions will be licensed under the [AGPL-3.0 License](LICENSE).

---

Thank you for contributing to Sablier! 🚀
