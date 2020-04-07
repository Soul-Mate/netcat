package util

import (
	"log"
	"sync/atomic"
	"time"
)

type Mesure struct {
	n uint64
}

func NewMesure() *Mesure {
	v := new(Mesure)
	atomic.StoreUint64(&v.n, 0)
	return v
}

func (m *Mesure) Add(n uint64) {
	atomic.AddUint64(&m.n, n)
}

func (m *Mesure) Set(n uint64) {
	atomic.StoreUint64(&m.n, n)
}

func (m *Mesure) GetAndSet(n uint64) uint64 {
	ret := atomic.LoadUint64(&m.n)
	atomic.StoreUint64(&m.n, n)
	return ret
}

func (m *Mesure) Run(interval time.Duration) {
	if interval == 0 {
		interval = time.Second // 每秒测量一次吞吐
	}

	for {
		now := time.Now()
		time.Sleep(interval)
		elapsed := time.Since(now)
		nBytes := m.GetAndSet(0)
		if nBytes > 0 {
			mib := float64(nBytes) / (1024.0 * 1024) / elapsed.Seconds()
			log.Printf("%.3f MiB/s\n", mib)
		}
	}
}
