package files

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/storage"
	"io"
	"sync"
	"time"
)

const (
	defaultWorkerPoolSize    = 5
	defaultWorkerPoolMaxSize = 20
)

type cRange struct {
	key    string
	index  int64
	offset int64
	limit  int64
	data   io.ReadCloser
}

type pool struct {
	s       storage.Storage
	workers []*worker
	read    bool
	write   bool
	mux     sync.Mutex
}

func (p *pool) dispatch(ctx context.Context, c *cRange) (int, error) {
	p.mux.Lock()
	defer p.mux.Unlock()
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			idx := -1
			for i, w := range p.workers {
				if w.status == workerStatReady {
					idx = i
					break
				}
			}

			if idx == -1 {
				if len(p.workers) == defaultWorkerPoolMaxSize {
					time.Sleep(time.Millisecond * 50)
					continue
				}
				p.workers = append(p.workers, initWorker(p.s, p.read, p.write))
				idx = len(p.workers) - 1
			}

			p.workers[idx].cRange = c
			p.workers[idx].status = workerStatRunning
			p.workers[idx].signalCh <- struct{}{}

			return idx, nil
		}
	}
}

func (p *pool) wait(ctx context.Context, idx int) (*cRange, error) {
	if idx >= len(p.workers) {
		return nil, fmt.Errorf("worker not found")
	}
	t := time.NewTicker(time.Millisecond * 100)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.C:
			switch p.workers[idx].status {
			case workerStatDone:
				result := p.workers[idx].cRange
				p.mux.Lock()
				p.workers[idx].reset()
				p.mux.Unlock()
				return result, nil
			case workerStatError:
				result := p.workers[idx].cRange
				message := p.workers[idx].message
				p.mux.Lock()
				p.workers[idx].reset()
				p.mux.Unlock()
				return result, fmt.Errorf(message)
			}
		}
	}
}

func (p *pool) close(ctx context.Context) error {
	p.mux.Lock()
	defer p.mux.Unlock()
	for idx := range p.workers {
		p.workers[idx].close()
	}
	return nil
}

func newReadWorkerPool(s storage.Storage) *pool {
	return newWorkerPool(s, true, false)
}

func newWriteWorkerPool(s storage.Storage) *pool {
	return newWorkerPool(s, false, true)
}

func newWorkerPool(s storage.Storage, isRead, isWrite bool) *pool {
	workers := make([]*worker, defaultWorkerPoolSize)
	for i := 0; i < defaultWorkerPoolSize; i += 1 {
		workers[i] = initWorker(s, isRead, isWrite)
	}
	return &pool{s: s, workers: workers}
}

const (
	workerStatReady = iota
	workerStatDone
	workerStatError
	workerStatRunning
	workerStatClosed
)

type worker struct {
	storage.Storage

	cRange   *cRange
	status   int
	signalCh chan struct{}
	message  string

	read  bool
	write bool
}

func (w *worker) run() {
	running := true
	for {
		_, running = <-w.signalCh
		if !running {
			return
		}

		if w.cRange == nil || w.status != workerStatRunning {
			continue
		}

		var err error
		switch {
		case w.read:
			err = w.doRead()
		case w.write:
			err = w.doWrite()
		}
		if err != nil {
			w.status = workerStatError
			w.message = err.Error()
			continue
		}
		w.status = workerStatDone
	}
}

func (w *worker) close() {
	w.status = workerStatClosed
	close(w.signalCh)
}

func (w *worker) doRead() error {
	var err error
	w.cRange.data, err = w.Get(context.TODO(), w.cRange.key, w.cRange.index, w.cRange.offset, w.cRange.limit)
	return err
}

func (w *worker) doWrite() error {
	defer w.cRange.data.Close()
	return w.Put(context.TODO(), w.cRange.key, w.cRange.data, w.cRange.index, w.cRange.offset)
}

func (w *worker) reset() {
	w.cRange = nil
	w.status = workerStatReady
	w.message = ""
}

func initWorker(s storage.Storage, isRead, isWrite bool) *worker {
	w := &worker{
		Storage:  s,
		write:    isWrite,
		read:     isRead,
		signalCh: make(chan struct{}),
	}
	w.reset()
	go w.run()
	return w
}
