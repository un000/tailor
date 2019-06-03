// Copyright 2019 Yegor Myskin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tailor

import (
	"testing"
	"time"
)

func TestChannelBasedRateLimiterDisallow(t *testing.T) {
	l := NewChannelBasedRateLimiter(30)
	defer l.Close()

	start := time.Now()
	for i := 0; i < 100; i++ {
		if !l.Allow() {
			t.FailNow()
		}
	}
	dur := time.Since(start)

	if dur < 3333*time.Millisecond || dur > 3500*time.Millisecond {
		t.Errorf("expected duration: ~3.33s, actual: %s", dur)
	}
}
