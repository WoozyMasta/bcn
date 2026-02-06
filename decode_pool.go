package bcn

import "sync"

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

type decodePool struct {
	jobs chan decodeJob
}

func newDecodePool(workers int) *decodePool {
	pool := &decodePool{
		jobs: make(chan decodeJob, workers),
	}
	for i := 0; i < workers; i++ {
		go pool.worker()
	}
	return pool
}

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
				for i := 0; i < 16; i++ {
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
	decodePoolsMu sync.Mutex
	decodePools   = map[int]*decodePool{}
)

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
