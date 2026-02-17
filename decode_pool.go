// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "sync"

// decodeJob carries a contiguous block range for parallel decoding.
type decodeJob struct {
	wg        *sync.WaitGroup
	data      []byte
	out       []byte
	start     int
	end       int
	bx        int
	width     int
	height    int
	blockSize int
	format    Format
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
		for idx := job.start; idx < job.end; idx++ {
			pos := idx * job.blockSize
			x := idx % job.bx
			y := idx / job.bx
			var block [16]rgba8

			switch job.format {
			case FormatDXT1:
				block = decodeBlockDXT1(job.data[pos : pos+8])
			case FormatDXT3:
				block = decodeBlockDXT3(job.data[pos : pos+16])
			case FormatDXT5:
				block = decodeBlockDXT5(job.data[pos : pos+16])
			case FormatBC4:
				alpha := decodeBlockBC4(job.data[pos : pos+8])
				for i := range 16 {
					block[i] = rgba8{r: alpha[i], g: alpha[i], b: alpha[i], a: 255}
				}
			case FormatBC5:
				block = decodeBlockBC5(job.data[pos : pos+16])
			}

			storeBlock(job.out, job.width, job.height, x, y, block)
		}
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
