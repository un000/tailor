// Copyright 2019 Yegor Myskin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tailor

// RateLimiter provides methods to create a custom rate limiter.
type RateLimiter interface {
	// Allow says that a line should be sent to a receiver of a lines.
	Allow() bool

	// Close finalizes rate limiter.
	Close()
}
