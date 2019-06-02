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

	err := t.Run(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("Tailing file:", t.FileName())
	for {
		select {
		case line, ok := <-t.Lines():
			if !ok {
				return
			}

			fmt.Println(line.StringTrimmed())
		case err, ok := <-t.Errors():
			if !ok {
				return
			}

			panic(err)
		}
	}
}
