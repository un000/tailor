// Copyright 2019 Yegor Myskin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tailor provides the functionality of tailing nginx_access.log under logrotate.
// Tailor can follow a selected log file and reopen it. Now, tailor doesn't require inotify, because it polls logs
// with a tiny delay. So the library can achieve cross-platform.
package tailor
