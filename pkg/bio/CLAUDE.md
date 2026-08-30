# pkg/bio

## Purpose

Block I/O layer for NanaFS: buffered read/write access to file data through a 4MB-page LFU cache and 64MB chunk store. Handles write coalescing into segments, segment-tree-based data location, local cache management, and optional chunk compaction.

## Key Files

| File | Responsibility |
|------|----------------|
| `bio.go` | `Reader`/`Writer` interfaces, global `CloseAll()`, `InvalidCacheHook` type |
| `chunk.go` | Core implementation: `ChunkStore` interface, `chunkReader`/`chunkWriter`, `segReader`/`segWriter`, `uncommittedSeg`/`uncommittedPage`, `segTree` interval tree, `CompactChunksData`/`DeleteChunksData`, `Options`/`Option` hooks |
| `pagecache.go` | LFU `pageCache` with radix-like `pageRoot`/`pageNode` directory (64 slots, 6-bit shift), `InitPageCache`, background eviction goroutine |
| `metric.go` | Prometheus histograms/counters/gauges for chunk I/O, commits, page cache |
| `utils.go` | `maxOff`, `minOff`, `expectPreRead` helpers |
| `origin.go` | Stub `originReader`/`originWriter` |

## Core Capabilities

**`Reader` interface:**
- `ReadAt(ctx, dest []byte, off int64) (n int64, error)`
- `Close()`

**`Writer` interface:**
- `WriteAt(ctx, data []byte, off int64) (n int64, error)`
- `Flush(ctx) error`
- `Fsync(ctx) error`
- `Close()`

**`ChunkStore` interface** (satisfied by `pkg/metastore`): `NextSegmentID`, `ListSegments`, `AppendSegments`, `DeleteSegment`.

**Options:** `WithCompactHook(fn func()) Option`; global `InitPageCache(sizeMB)` and `CloseAll()`.

## Upstream (Consumers)

- `pkg/core` (core.go, rawfile.go) — all file data I/O flows through bio readers/writers

## Downstream (Dependencies)

- `github.com/basenana/nanafs/pkg/storage` (`Storage`, `LocalCache`, `CacheNode`)
- `github.com/basenana/nanafs/pkg/types` (`ChunkSeg`, `Entry`)
- `github.com/basenana/nanafs/utils` (`LFUPool`, `ParallelWorker`, offset readers/writers, `ZeroDevice`, `Recover`)
- External: `go.uber.org/zap`, `prometheus/client_golang`

## Design Notes

- Page cache: LFU eviction, max 1024 pages × 4MB (≈4GB default); background goroutine evicts when cache exceeds 80% capacity.
- Segment tree: `segTree` is an interval tree over sorted `ChunkSeg` records; `query(start, end)` locates data for a read range, `cut(off)` splits at truncation points.
- Write coalescing: `segWriter` batches dirty pages into `uncommittedSeg`; commits via `AppendSegments` when a segment fills or after a 1-minute timeout.
- Chunk size 64MB, page size 4MB (configurable to 2MB via `InitPageCache(2)`).
- Bounded parallelism: 256 concurrent read tasks, 64 write tasks.
- `CompactChunksData` rewrites existing segments as one sequential segment (triggered by dispatch's `compactExecutor`).

## Testing

- `suite_test.go` — Ginkgo/Gomega suite (`TestBIO`) using `metastore.MemoryMeta` as ChunkStore and `storage.MemoryStorage` as data store.
- `chunk_test.go` — standard Go tests.
