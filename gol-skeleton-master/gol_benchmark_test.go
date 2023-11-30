package main

import (
	"fmt"
	"os"
	"testing"
	"uk.ac.bris.cs/gameoflife/gol"
)

func BenchmarkRun(b *testing.B) {

	os.Stdout = nil

	for threads := 1; threads <= 16; threads++ {

		params := gol.Params{
			Turns:       100,
			Threads:     threads,
			ImageWidth:  512,
			ImageHeight: 512,
		}

		b.Run(fmt.Sprintf("%d_threads", threads), func(b *testing.B) {

			for i := 0; i < b.N; i++ {


				events := make(chan gol.Event, 1000)
				//keyPresses := make(chan rune, 10)

				go gol.Run(params, events, nil)
				for range events {
				}
				//close(events)
				//close(keyPresses)
			}
		})
	}
}
