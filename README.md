Tailor, the library for tailing nginx access logs
-----
[![Go Doc](https://godoc.org/github.com/un000/tailor?status.svg)](https://godoc.org/github.com/un000/tailor)

Tailor provides the functionality of tailing nginx access log under logrotate.
Tailor will follow a selected log file and reopen it if it's been rotated. Now, tailor doesn't require inotify, because it polls logs
with a tiny delay. So the library can achieve cross-platform.

There is no plan to implement truncate detection.

Currently this library is used in production, handling 5k of opened files with a load over 100k rps per instance,
without such overhead.
![Actual usage](https://i.imgur.com/G4QICfk.png)

## Install
```
go get github.com/un000/tailor
```

## Example
```
package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/un000/tailor"
)

func main() {
	t := tailor.New(
		"./github.com_access.log",
		tailor.WithSeekOnStartup(0, io.SeekStart),
		tailor.WithPollerTimeout(10*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lines, errs, err := t.Run(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("Tailing file:", t.FileName())
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return
			}

			fmt.Println(line.StringTrimmed())
		case err, ok := <-errs:
			if !ok {
				return
			}

			panic(err)
		}
	}
}

```

## Contributions are appreciated, feel free ✌️