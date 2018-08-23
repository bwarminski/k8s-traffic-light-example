package util

import (
	"sync/atomic"
	"time"
)

type Clock interface {
	Now() int64
}

type RealClock struct{}

func (r *RealClock) Now() int64 {
	return time.Now().Unix()
}

type FakeClock struct {
	time int64
}

func (f *FakeClock) Now() int64 {
	return atomic.LoadInt64(&f.time)
}

func (f *FakeClock) Set(t int64) {
	atomic.StoreInt64(&f.time, t)
}
