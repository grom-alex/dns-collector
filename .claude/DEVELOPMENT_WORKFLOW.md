# DNS Collector - Development Workflow

This document describes the mandatory development workflow for this project. These steps must be followed before pushing any code changes to the main codebase.

## Mandatory Pre-Push Workflow

Before pushing any changes to the main code, execute the following steps in order:

### 1. Run Lint Checks ✅

Lint checks must pass for both projects before proceeding.

```bash
# dns-collector
cd /home/test/projects/dns-collector/dns-collector
make lint

# web-api
cd /home/test/projects/dns-collector/web-api
make lint
```

**Success criteria**: No lint errors or warnings from golangci-lint.

### 2. Run Unit Tests ✅

All unit tests must pass before proceeding.

```bash
# dns-collector
cd /home/test/projects/dns-collector/dns-collector
make test-unit

# web-api
cd /home/test/projects/dns-collector/web-api
make test-unit
```

**Success criteria**: All tests pass with no failures.

### 3. Build Docker Images 🐳

Build Docker images with version tags. Use semantic versioning (e.g., 2.2.2).

```bash
# dns-collector
cd /home/test/projects/dns-collector/dns-collector
docker build --no-cache \
  -t registry.gromas.ru/apps/dns-collector/dns-collector:X.Y.Z \
  -t registry.gromas.ru/apps/dns-collector/dns-collector:latest .

# web-api
cd /home/test/projects/dns-collector/web-api
docker build --no-cache \
  -t registry.gromas.ru/apps/dns-collector/web-api:X.Y.Z \
  -t registry.gromas.ru/apps/dns-collector/web-api:latest .
```

**Success criteria**: Images build successfully without errors.

### 4. Test Docker Images Locally 🧪

Verify that Docker images work correctly.

```bash
# Test dns-collector
docker run --rm registry.gromas.ru/apps/dns-collector/dns-collector:X.Y.Z \
  ./dns-collector --help

# Test web-api
docker run --rm registry.gromas.ru/apps/dns-collector/web-api:X.Y.Z \
  ./web-api --help
```

**Success criteria**: Images start and respond correctly (database connection errors are expected without a database).

### 5. Publish Images to Registry 📦

Push images to the private registry.

```bash
# Push dns-collector
docker push registry.gromas.ru/apps/dns-collector/dns-collector:X.Y.Z
docker push registry.gromas.ru/apps/dns-collector/dns-collector:latest

# Push web-api
docker push registry.gromas.ru/apps/dns-collector/web-api:X.Y.Z
docker push registry.gromas.ru/apps/dns-collector/web-api:latest
```

**Success criteria**: Images successfully pushed to registry.

### 6. Update Production Configuration 📝

Update the production docker-compose.yml with new version numbers.

```bash
# Edit deploy/production/docker-compose.yml
# Update image versions for both services:
#   dns-collector: registry.gromas.ru/apps/dns-collector/dns-collector:X.Y.Z
#   web-api: registry.gromas.ru/apps/dns-collector/web-api:X.Y.Z
```

**Success criteria**: Production config updated with correct version numbers.

### 7. Create Git Tag and Push 🏷️

Commit changes, create an annotated tag, and push to remote.

```bash
# Stage all changes
git add -A

# Create commit with detailed description
git commit -m "Release vX.Y.Z: <Summary>

<Detailed description of changes>

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"

# Create annotated tag with full changelog
git tag -a vX.Y.Z -m "Release vX.Y.Z - <Title>

<Full changelog with sections:>
- Key Improvements
- Testing & Quality
- Configuration Updates
- Docker Images
- Files Changed
- Migration Notes
- Credits

🤖 Generated with [Claude Code](https://claude.com/claude-code)
Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"

# Push changes and tag
git push origin <branch-name>
git push origin vX.Y.Z
```

**Success criteria**: Changes and tag successfully pushed to remote repository.

## Development Environment

### System Information
- Package manager: `apt`
- If additional system utilities are needed, request user installation via apt
- golangci-lint is installed locally at version 1.64.8

### Available Make Targets

Both projects (dns-collector and web-api) have the following Make targets:

- `make lint` - Run golangci-lint with 5m timeout
- `make test-unit` - Run unit tests with race detector
- `make test-coverage` - Run tests with coverage report
- `make build` - Build the binary
- `make clean` - Clean build artifacts

## Version Numbering

Follow semantic versioning (MAJOR.MINOR.PATCH):
- MAJOR: Breaking changes
- MINOR: New features, backward compatible
- PATCH: Bug fixes and improvements

## Git Commit Messages

All commits should include:
1. Clear summary line (50 chars or less)
2. Detailed description of changes
3. Credits footer with Claude Code attribution

## Docker Images

Registry: `registry.gromas.ru/apps/dns-collector/`

Images:
- `dns-collector:X.Y.Z` (latest: ~96.4MB)
- `web-api:X.Y.Z` (latest: ~93.5MB)

Always tag with both specific version and `latest`.

## Testing Requirements

### Unit Tests
- All new code must have unit tests
- Target coverage: 80%+ for handlers and critical paths
- Use table-driven tests for multiple scenarios
- Include edge cases and error conditions

### Test Organization
```go
func TestFeatureName(t *testing.T) {
    tests := []struct {
        name        string
        input       Type
        expected    Type
        expectError bool
    }{
        {"valid case", validInput, expectedOutput, false},
        {"error case", invalidInput, nil, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Configuration Management

### Configuration Files
- `dns-collector/config/config.yaml` - Production defaults
- `dns-collector/config/config.dev.yaml` - Development settings
- `deploy/production/docker-compose.yml` - Production deployment

### Configuration Validation
All configuration must be validated in `internal/config/config.go`:
- Set sensible defaults
- Validate ranges and constraints
- Return clear error messages

Example:
```go
if cfg.Retention.StatsDays <= 0 {
    cfg.Retention.StatsDays = 30 // default
}
if cfg.Retention.StatsDays > 365 {
    return nil, fmt.Errorf("retention stats_days must not exceed 365 days, got %d", cfg.Retention.StatsDays)
}
```

## Code Quality Standards

### Error Handling
- Never ignore errors with `_ = ...`
- Always log errors appropriately
- Use structured error messages with context

Good:
```go
defer func() {
    if err := db.Close(); err != nil {
        log.Printf("Error closing database: %v", err)
    }
}()
```

Bad:
```go
defer func() { _ = db.Close() }()
```

### Logging
- Use appropriate log levels (debug, info, warn, error)
- Include context in log messages
- Log important operations and errors

## Recent Changes Log

### v2.2.2 (2025-12-22)
- Added error logging in defer closures for database connections
- Added configuration validation for retention settings
- Made cleanup interval configurable via `retention.cleanup_interval_hours`
- Added 9 new tests for configuration validation
- All tests passing (58+ tests total)

### v2.2.1 (Previous)
- Fixed pagination and filtering issues
- Added pull_policy: always for services
- Improved IP display in web UI
- Fixed web UI crashes on empty/invalid filter results

## Important Notes

1. **Never skip the workflow steps** - Each step is critical for production stability
2. **Always run tests locally** before pushing to avoid CI failures
3. **Tag every release** with detailed changelog for traceability
4. **Update production config** to match deployed versions
5. **Document breaking changes** clearly in release notes

## Troubleshooting

### Lint Failures
```bash
# Check specific issues
golangci-lint run --timeout=5m

# Auto-fix some issues
golangci-lint run --fix
```

### Test Failures
```bash
# Run specific test
go test -v -run TestName ./path/to/package

# Run with race detector
go test -race ./...

# Check coverage
go test -cover ./...
```

### Docker Build Issues
```bash
# Clean Docker cache
docker system prune -a

# Rebuild without cache
docker build --no-cache -t image:tag .
```

---

**Last Updated**: 2025-12-22
**Maintained By**: Development Team + Claude Code
