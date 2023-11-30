package gol

import (
	"fmt"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

func countNeighbours(y, x int, world [][]uint8, p Params) int {
	neighbours := 0
	for y1 := -1; y1 <= 1; y1++ {
		for x1 := -1; x1 <= 1; x1++ {
			if world[(y+y1+p.ImageHeight)%p.ImageHeight][(x+x1+p.ImageWidth)%p.ImageWidth] == 255 && (y1 != 0 || x1 != 0) {
				neighbours++
			}
		}
	}
	return neighbours
}

func updateWorld(startY, endY int, world, worldUpdate [][]uint8, p Params) [][]uint8 {
	for y := startY; y < endY; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			neighbours := countNeighbours(y, x, world, p)

			if world[y][x] == 255 {
				if neighbours > 3 || neighbours < 2 {
					worldUpdate[y][x] = 0
				}
			} else {
				if neighbours == 3 {
					worldUpdate[y][x] = 255
				}
			}
		}
	}
	return worldUpdate
}

func worker(startY, endY int, world, worldUpdate [][]uint8, out chan<- [][]uint8, p Params) {
	newWorld := updateWorld(startY, endY, world, worldUpdate, p)
	out <- newWorld
}

func countAliveCells(world [][]uint8) int {
	count := 0
	for y := range world {
		for x := range world[y] {
			if world[y][x] == 255 {
				count++
			}
		}
	}
	return count
}

func saveWorldToPGM(world [][]uint8, c distributorChannels, p Params, turns int) {
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprint(p.ImageWidth) + "x" + fmt.Sprint(p.ImageHeight) + "x" + fmt.Sprint(turns)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// Wait for the io goroutine to finish writing the image
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprint(p.ImageWidth) + "x" + fmt.Sprint(p.ImageHeight)

	world := make([][]uint8, p.ImageHeight)
	worldUpdate := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
		worldUpdate[i] = make([]uint8, p.ImageWidth)
	}
	count := 0
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			val := <-c.ioInput
			count++
			world[y][x] = val
			worldUpdate[y][x] = val
		}
	}

	turn := 0

	// Send CellFlipped events for initially alive cells
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
			}
		}
	}

	var mutex sync.Mutex // Adding mutex to ensure ticker and updateWorld don't access world at the same time

	// Create ticker and a tickerPause channel
	ticker := time.NewTicker(2 * time.Second)
	done := make(chan struct{})
	tickerPaused := false
	// TODO: Execute all turns of the Game of Life.

	go func() {
		for {
			select {
			case <-ticker.C:
				if !tickerPaused {
					mutex.Lock()                         // Lock before accessing the world
					aliveCount := countAliveCells(world) // Function to count alive cells
					mutex.Unlock()                       // Unlock after accessing
					aliveCells := AliveCellsCount{turn, aliveCount}
					c.events <- aliveCells
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	paused := false
	quit := false

	for turn < p.Turns && !quit {
		var newWorld [][]uint8
		select {
		case key := <-c.keyPresses:
			switch key {
			case 'p':
				paused = !paused
				if paused {
					c.events <- StateChange{turn, 0}
					tickerPaused = true
				} else {
					c.events <- StateChange{turn, 1}
					tickerPaused = false
				}
			case 's':
				fmt.Println("Starting output")
				saveWorldToPGM(world, c, p, turn)
			case 'q':
				c.events <- StateChange{turn, 2}
				quit = true
			}
		default:
			if !paused {
				if p.Threads == 1 {
					newWorld = updateWorld(0, p.ImageHeight, world, worldUpdate, p)
				} else {
					in := make([]chan [][]uint8, p.Threads)
					threadHeight := p.ImageHeight / p.Threads
					tempHeight := 0
					if p.Threads <= p.ImageHeight {
						for i := 0; i < p.Threads; i++ {
							in[i] = make(chan [][]uint8)
							if i == p.Threads-1 {
								go worker(tempHeight, p.ImageHeight, world, worldUpdate, in[i], p)

							} else {
								if tempHeight+threadHeight+tempHeight%2 >= p.ImageHeight {
									go worker(tempHeight, p.ImageHeight, world, worldUpdate, in[i], p)
								} else {
									go worker(tempHeight, tempHeight+threadHeight+tempHeight%2, world, worldUpdate, in[i], p)
									tempHeight += threadHeight + tempHeight%2
								}
							}
						}
						for i := 0; i < p.Threads; i++ {
							newWorld = append(newWorld, <-in[i]...)
						}
					}
				}
				// Send CellFlipped events for cells that change state
				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						if world[y][x] != newWorld[y][x] {
							c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
						}
					}
				}

				// Update the world array after each turn
				mutex.Lock() // Lock before modifying the world
				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						world[y][x] = newWorld[y][x]
					}
				}
				mutex.Unlock() // Unlock after modifying

				// Send TurnComplete event after updating the world
				c.events <- TurnComplete{CompletedTurns: turn}

				turn++
			}
		}
	}

	// TODO: Create a 2D slice to store the world.
	saveWorldToPGM(world, c, p, turn)

	var alive []util.Cell
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				alive = append(alive, util.Cell{x, y})
			}
		}
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	output := FinalTurnComplete{turn, alive}
	c.events <- output

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
	close(done)
}
