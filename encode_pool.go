// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "sync"

// encodeJob carries a contiguous block range for parallel encoding.
type encodeJob struct {
	rgba    []byte
	out     []byte
	wg      *sync.WaitGroup
	options EncodeOptions
	format  Format
	start   int
	end     int
	bx      int
	width   int
	height  int
}

// encodePool keeps long-lived workers to avoid per-call goroutine churn.
type encodePool struct {
	jobs chan encodeJob
}

// newEncodePool starts a fixed worker set for one worker-count bucket.
func newEncodePool(workers int) *encodePool {
	pool := &encodePool{
		jobs: make(chan encodeJob, workers),
	}
	for range workers {
		go pool.worker()
	}
	return pool
}

// worker encodes its assigned block range and signals completion via WaitGroup.
func (p *encodePool) worker() {
	for job := range p.jobs {
		// Format is validated by the driver before jobs are submitted.
		_ = encodeBlockRange(job.format, job.rgba, job.out, job.width, job.height, job.bx, job.start, job.end, job.options)
		job.wg.Done()
	}
}

var (
	// encodePoolsMu protects encodePools map by worker-count key.
	encodePoolsMu sync.Mutex
	// encodePools reuses worker pools across calls with the same worker count.
	encodePools = map[int]*encodePool{}
)

// getEncodePool returns (or creates) a reusable pool for the requested worker count.
func getEncodePool(workers int) *encodePool {
	encodePoolsMu.Lock()
	defer encodePoolsMu.Unlock()

	pool := encodePools[workers]
	if pool == nil {
		pool = newEncodePool(workers)
		encodePools[workers] = pool
	}

	return pool
}
