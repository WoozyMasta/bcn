// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "sync"

// decodeJob carries a contiguous block range for parallel decoding.
type decodeJob struct {
	wg     *sync.WaitGroup
	data   []byte
	out    []byte
	format Format
	start  int
	end    int
	bx     int
	width  int
	height int
}

// decodePool keeps long-lived workers to avoid per-call goroutine churn.
type decodePool struct {
	jobs chan decodeJob
}

// newDecodePool starts a fixed worker set for one worker-count bucket.
func newDecodePool(workers int) *decodePool {
	pool := &decodePool{
		jobs: make(chan decodeJob, workers),
	}
	for range workers {
		go pool.worker()
	}
	return pool
}

// worker decodes its assigned block range and signals completion via WaitGroup.
func (p *decodePool) worker() {
	for job := range p.jobs {
		// Format is validated by the driver before jobs are submitted.
		_ = decodeBlockRange(job.format, job.data, job.out, job.width, job.height, job.bx, job.start, job.end)
		job.wg.Done()
	}
}

var (
	// decodePoolsMu protects decodePools map by worker-count key.
	decodePoolsMu sync.Mutex
	// decodePools reuses worker pools across calls with the same worker count.
	decodePools = map[int]*decodePool{}
)

// getDecodePool returns (or creates) a reusable pool for the requested worker count.
func getDecodePool(workers int) *decodePool {
	decodePoolsMu.Lock()
	defer decodePoolsMu.Unlock()

	pool := decodePools[workers]
	if pool == nil {
		pool = newDecodePool(workers)
		decodePools[workers] = pool
	}

	return pool
}
