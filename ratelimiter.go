package downloader

import (
	"time"
)

// Limiter is rate limiter interface.
type Limiter interface {
	// Wait handles the limit logics.
	Wait(readNum int64)

	// LimitNum returns the limit num.
	LimitNum() int64

	// Reset resets limits.
	Reset(newLimitNum int64)
}

// SimpleRateLimiter is simple rate limiter.
type SimpleRateLimiter struct {
	// limitNum is target num of limit.
	limitNum int64

	// readNum is num of read datas in each check.
	readNum int64

	// lastTS is last check timestamp.
	lastTS time.Time
}

// NewSimpleRateLimiter creates a new SimpleRateLimiter object base on target limit num.
func NewSimpleRateLimiter(limitNum int64) *SimpleRateLimiter {
	return &SimpleRateLimiter{limitNum: limitNum}
}

// Wait handles simple rate limiter limit logics.
func (l *SimpleRateLimiter) Wait(readNum int64) {
	if (time.Now().UnixNano() - l.lastTS.UnixNano()) <= int64(time.Second) {
		num := readNum - l.readNum

		// check limit num.
		if num > l.limitNum {
			wait := time.Second.Nanoseconds() - (time.Now().UnixNano() - l.lastTS.UnixNano())
			time.Sleep(time.Duration(wait))

			// update after limit action.
			l.readNum = readNum
			l.lastTS = time.Now()
		}
	} else {
		// not limit, update.
		l.readNum = readNum
		l.lastTS = time.Now()
	}
}

// LimitNum returns the limit num.
func (l *SimpleRateLimiter) LimitNum() int64 {
	return l.limitNum
}

// Reset resets a new limit num.
func (l *SimpleRateLimiter) Reset(newLimitNum int64) {
	l.limitNum = newLimitNum
}
