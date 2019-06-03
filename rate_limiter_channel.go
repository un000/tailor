// Copyright 2019 Yegor Myskin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tailor

import (
	"time"
)

type ChannelBasedRateLimiter struct {
	t *time.Ticker
}

// NewChannelBasedRateLimiter creates an instance of rate limiter, which ticker ticks every period to limit the lps.
func NewChannelBasedRateLimiter(lps int) *ChannelBasedRateLimiter {
	return &ChannelBasedRateLimiter{
		t: time.NewTicker(time.Second / time.Duration(lps)),
	}
}

// Allow will block until the ticker ticks.
func (rl *ChannelBasedRateLimiter) Allow() bool {
	_, ok := <-rl.t.C
	return ok
}

func (rl *ChannelBasedRateLimiter) Close() {
	rl.t.Stop()
}
