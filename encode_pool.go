// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/bcn

package bcn

import "sync"

// encodeJob carries a contiguous block range for parallel encoding.
type encodeJob struct {
	wg        *sync.WaitGroup
	rgba      []byte
	out       []byte
	options   EncodeOptions
	start     int
	end       int
	bx        int
	width     int
	height    int
	blockSize int
	format    Format
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
		for idx := job.start; idx < job.end; idx++ {
			x := idx % job.bx
			y := idx / job.bx
			block := extractBlock(job.rgba, job.width, job.height, x, y)
			pos := idx * job.blockSize

			switch job.format {
			case FormatDXT1:
				b := encodeBlockDXT1WithOptions(block, job.options)
				copy(job.out[pos:pos+8], b[:])
			case FormatDXT3:
				b := encodeBlockDXT3WithOptions(block, job.options)
				copy(job.out[pos:pos+16], b[:])
			case FormatDXT5:
				b := encodeBlockDXT5WithOptions(block, job.options)
				copy(job.out[pos:pos+16], b[:])
			case FormatBC4:
				b := encodeBlockBC4(block, job.options, func(c rgba8) uint8 { return c.r })
				copy(job.out[pos:pos+8], b[:])
			case FormatBC5:
				b := encodeBlockBC5(block, job.options)
				copy(job.out[pos:pos+16], b[:])
			}
		}
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
