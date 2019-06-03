// Copyright 2019 Yegor Myskin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tailor

import (
	"io"
	"time"
)

// options is the main options store.
// To see what's going on, watch for With* descriptions below.
type options struct {
	runOffset int64
	runWhence int

	reopenOffset int64
	reopenWhence int

	bufioReaderPoolSize int
	pollerTimeout       time.Duration
	updateLagInterval   time.Duration

	rateLimiter RateLimiter
	leakyBucket bool
}

// withDefaultOptions sets the initial options.
func withDefaultOptions() []Option {
	return []Option{
		WithPollerTimeout(10 * time.Millisecond),
		WithReaderInitialPoolSize(4096),
		WithSeekOnStartup(0, io.SeekEnd),
		WithSeekOnReopen(0, io.SeekStart),
		WithUpdateLagInterval(5 * time.Second),
	}
}

type Option func(options *options)

// WithPollerTimeout is used to timeout when file is fully read, to check changes.
func WithPollerTimeout(duration time.Duration) Option {
	return func(options *options) {
		options.pollerTimeout = duration
	}
}

// WithReaderInitialPoolSize is used to set the internal initial size of bufio.Reader buffer.
func WithReaderInitialPoolSize(size int) Option {
	return func(options *options) {
		options.bufioReaderPoolSize = size
	}
}

// WithSeekOnStartup is used to set file.Seek() options, when the file is opened on startup.
// Use io.Seek* constants to set whence.
func WithSeekOnStartup(offset int64, whence int) Option {
	return func(options *options) {
		options.runOffset = offset
		options.runWhence = whence
	}
}

// WithSeekOnRun is used to set file.Seek() options, when the file is opened on startup.
// Use io.Seek* constants to set whence.
func WithSeekOnReopen(offset int64, whence int) Option {
	return func(options *options) {
		options.reopenOffset = offset
		options.reopenWhence = whence
	}
}

// WithUpdateLagInterval is used to know how often update the file lag.
// Frequent update time increasing Seek syscall calls.
func WithUpdateLagInterval(duration time.Duration) Option {
	return func(options *options) {
		options.updateLagInterval = duration
	}
}

// WithRateLimiter is used to rate limit output lines. Watch RateLimiter interface.
func WithRateLimiter(rl RateLimiter) Option {
	return func(options *options) {
		options.rateLimiter = rl
	}
}

// WithLeakyBucket is used to skip a read lines, when a listener is full.
func WithLeakyBucket() Option {
	return func(options *options) {
		options.leakyBucket = true
	}
}
