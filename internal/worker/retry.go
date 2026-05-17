package worker

import (
	"math"
	"math/rand/v2"
	"time"
)

const (
	baseDelay = 2 * time.Second
	maxDelay  = 5 * time.Minute
)

func Backoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	base := float64(baseDelay) * math.Pow(2, float64(attempt))
	delay := time.Duration(rand.Float64() * base)
	if delay > maxDelay {
		return maxDelay
	}

	return delay
}
