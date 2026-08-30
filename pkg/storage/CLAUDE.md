# pkg/storage

## Purpose

Abstracted block storage layer for NanaFS. Defines the `Storage` interface with six backend implementations (S3, OSS, MinIO, WebDAV, local filesystem, in-memory), a file-backed `LocalCache` with optional AES-CTR encryption and LZ4 compression, Prometheus instrumentation, and per-backend rate limiting.

## Key Files

| File | Responsibility |
|------|----------------|
| `storage.go` | `Storage` interface, `Info` struct, `NewStorage()` factory |
| `s3.go` | AWS S3 implementation (`s3Storage`) |
| `oss.go` | AlibabaCloud OSS implementation (`aliyunOSSStorage`) |
| `minio.go` | MinIO/S3-compatible implementation (`minioStorage`) |
| `webdav.go` | WebDAV implementation (`webdavStorage`) |
| `local.go` | Local filesystem (`local`) and in-memory (`memoryStorage`) implementations |
| `cache.go` | `LocalCache` with file-backed cache, encryption/compression, `CacheNode` interface, LRU priority-queue eviction (`cachedFileMapper`) |
| `instrumental.go` | `instrumentalStorage` wrapper adding Prometheus latency/error metrics |
| `utils.go` | LZ4 `compress()`/`decompress()`, AES-CTR `encrypt()`/`decrypt()`, key/IV helpers, `priorityNodeQueue` heap |

## Core Capabilities

**`Storage` interface:**
- `ID() string`
- `Get(ctx, key string, idx int64) (io.ReadCloser, error)`
- `Put(ctx, key string, idx int64, dataReader io.Reader) error`
- `Delete(ctx, key int64) error`
- `Head(ctx, key, idx int64) (Info, error)`

**`LocalCache`:**
- `NewLocalCache(s Storage) *LocalCache`
- `OpenTemporaryNode(ctx, oid, off int64) (CacheNode, error)`
- `CommitTemporaryNode(ctx, segID, idx int64, node CacheNode) error`
- `OpenCacheNode(ctx, key, idx int64) (CacheNode, error)`

**`CacheNode` interface:** `io.ReaderAt`, `io.WriterAt`, `io.Closer`, `Size() int64`.

**Factory:** `NewStorage(storageID, storageType string, cfg config.Storage) (Storage, error)` — switches on type constants `S3Storage`, `OSSStorage`, `MinioStorage`, `WebdavStorage`, `LocalStorage`, `MemoryStorage`.

## Upstream (Consumers)

- `pkg/core` (core.go, rawfile.go) — object storage for entry data
- `pkg/bio` (chunk.go, pagecache.go) — chunk/segment data plane
- Test suites: `pkg/core`, `pkg/bio`, `workflow/jobrun`, `cmd/apps/apis/*`

## Downstream (Dependencies)

- `github.com/basenana/nanafs/config`, `pkg/types`, `utils`, `utils/logger`
- External: `aws/aws-sdk-go-v2` (S3), `aliyun/aliyun-oss-go-sdk`, `minio/minio-go/v7`, `studio-b12/gowebdav`, `prometheus/client_golang`, `pierrec/lz4/v4`

## Design Notes

- Factory pattern (`NewStorage`) keyed on storage type string from config.
- Decorator pattern: `instrumentalStorage` wraps any backend to add Prometheus metrics.
- Local cache: LRU eviction via a priority-queue heap; AES-CTR keys derived from segment ID (SHA256); LZ4 compression applied in a pipelined goroutine model.
- Rate limiting: each backend honors read/write concurrency limits via env vars (e.g. `STORAGE_S3_READ_LIMIT`).
- Object keys are reverse-string hashed for prefix distribution across object-store partitions.
- S3 uses adaptive retry mode with up to 50 attempts; other backends have inline retry loops.

## Testing

- `suite_test.go` — Ginkgo/Gomega suite entry `TestStorage`.
- `cache_test.go` — temporary/cache node tests incl. encryption.
- `utils_test.go` — helper tests.
