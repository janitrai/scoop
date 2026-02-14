package globaltime

import (
	"sync"
	"time"
)

var (
	mu      sync.RWMutex
	nowFunc = time.Now
)

func Now() time.Time {
	mu.RLock()
	defer mu.RUnlock()
	return nowFunc()
}

func UTC() time.Time {
	return Now().UTC()
}

func SetMockTime(t time.Time) {
	mu.Lock()
	defer mu.Unlock()
	nowFunc = func() time.Time { return t }
}

func ResetTime() {
	mu.Lock()
	defer mu.Unlock()
	nowFunc = time.Now
}
