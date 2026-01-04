# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

NanaFS is a **Reference Filing System** inspired by GTD methodology, designed as a file-centric workflow engine for unified data management. It provides cloud-based storage with multiple backend support, POSIX-compatible file system interface via FUSE, and a workflow engine with rule-based processing.

## Build and Development Commands

### Building
- `make build` - Build multi-architecture binaries using Docker (Linux amd64/arm64, Darwin amd64/arm64)
- `make docker-build` - Build Docker image for Linux/amd64
- `make docker-buildx` - Build and push multi-platform Docker images (requires Docker Buildx)
- `make buildbin` - Build for specific OS/ARCH (usage: `make buildbin GOOS=linux GOARCH=amd64`)

### Testing and Quality
- `make test` - Run all Go tests (uses Ginkgo/Gomega for BDD-style testing)
- `make check` - Format code and run `go vet`
- `make lint` - Run golangci-lint with all checks enabled
- `go test ./pkg/...` - Run tests for specific package
- `go test -v ./pkg/core` - Run verbose tests for core package

### Code Generation
- `make fsapi` - Generate gRPC code from protobuf definitions (located in `cmd/apps/apis/fsapi/v1/fsapi-v1.proto`)

### Cleanup
- `make clean` - Clean workspace
- `go clean ./...` - Clean all packages

## Architecture Overview

### Core Components

**Entry Management (`pkg/core/`)**
- Central file system logic handling namespaces, entries, and metadata
- Entry lifecycle management and caching
- Interface: `core.Core` provides main API for file operations

**Storage Backends (`pkg/storage/`)**
- Abstracted storage interface supporting multiple providers:
  - Object storage: AWS S3, AlibabaCloud OSS, MinIO
  - File hosting: WebDAV
  - Local filesystem
- Interface: `storage.Storage` with provider-specific implementations

**Metadata Store (`pkg/metastore/`)**
- PostgreSQL and SQLite support via GORM
- Schema migrations using gormigrate
- Centralized metadata management for entries and workflows

**Workflow Engine (`workflow/`, `pkg/cel/`)**
- File-centric workflow processing with rule-based automation
- CEL expression evaluation for rule conditions
- Integration with `github.com/basenana/go-flow`

**Plugin System (`pkg/plugin/`)**
- Extensible plugin architecture for:
  - Source plugins (RSS, SMTP email aggregation)
  - Process plugins (file processing in workflows)

**Event System (`pkg/events/`)**
- Event bus for workflow triggers and system notifications
- Decoupled architecture for extensibility

### Application Structure

**CLI Entry Points (`cmd/`)**
- `cmd/main.go` - Main application entry point
- `cmd/apps/` - Cobra-based CLI structure:
  - `apis/` - API servers (FS API, WebDAV)
  - `fuse/` - FUSE file system implementation

**Configuration (`config/`)**
- JSON-based configuration loading
- Support for multiple storage backends and metadata stores
- API endpoint configuration (FS API, WebDAV, FUSE)

### Key Design Patterns

1. **Interface-based Design** - All major components use interfaces for testability and extensibility
2. **Event-driven Architecture** - Workflows triggered by file events via event bus
3. **Namespace Isolation** - Multi-tenancy support through namespace separation
4. **Caching Layer** - Page cache in `pkg/bio/` for performance optimization
5. **Plugin Extensibility** - Hot-pluggable plugins for custom functionality

## Development Notes

### Dependencies
- Go 1.23+ required (see `go.mod`)
- Vendored dependencies in `vendor/` directory
- Major dependencies: go-fuse/v2, gin-gonic/gin, gorm, various cloud SDKs

### Testing
- BDD-style tests using Ginkgo/Gomega
- Test files follow `*_test.go` naming convention
- `suite_test.go` files for test suite setup
- E2E tests in `e2e/` directory including POSIX compliance tests

### Code Style
- Use `go fmt` and `go vet` (enforced by `make check`)
- Follow standard Go conventions and idioms
- Structured logging with zap (`utils/logger/`)
- Error handling follows Go conventions with `pkg/errors` for wrapping

### Release Process
- Automated releases via GoReleaser (`.goreleaser.yaml`)
- Multi-architecture builds for Linux and Darwin
- Docker images published to registry.cn-hangzhou.aliyuncs.com
- GitHub Actions workflows for CI/CD

## Common Development Tasks

### Adding a New Storage Backend
1. Implement `storage.Storage` interface in `pkg/storage/`
2. Add configuration support in `config/`
3. Register backend in storage factory
4. Add tests for new implementation

### Creating a Workflow Plugin
1. Implement plugin interface in `pkg/plugin/`
2. Register plugin in plugin registry
3. Add workflow step type definition
4. Test with sample workflow configuration

### Modifying Core File Operations
1. Update interfaces in `pkg/core/`
2. Modify implementation in `pkg/core/core.go`
3. Update affected tests
4. Consider impact on caching and event system

### Adding API Endpoints
1. Define protobuf messages (if gRPC) or Gin routes
2. Implement handler in `cmd/apps/apis/`
3. Add middleware for authentication/authorization
4. Update API documentation

## Configuration

Configuration is JSON-based with support for:
- Multiple storage backends with credentials
- Metadata store selection (PostgreSQL/SQLite)
- API server endpoints and ports
- Workflow engine settings
- Encryption and security settings

See `config/` directory for configuration structure and examples.