package downloader

import (
	"time"
)

// Limiter is rate limiter interface.
type Limiter interface {
	// Wait handles the limit logics.
	Wait(n int64)
}

// SimpleRateLimiter is simple rate limiter.
type SimpleRateLimiter struct {
	// LimitNum is target num of limit.
	LimitNum int64

	// readNum is num of read datas in each check.
	readNum int64

	// lastTS is last check timestamp.
	lastTS time.Time
}

// Wait handles simple rate limiter limit logics.
func (l *SimpleRateLimiter) Wait(readNum int64) {
	if (time.Now().UnixNano() - l.lastTS.UnixNano()) <= int64(time.Second) {
		num := readNum - l.readNum

		// check limit num.
		if num > l.LimitNum {
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
