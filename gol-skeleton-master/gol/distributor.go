package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
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

func saveWorldToPGM(world [][]uint8, c distributorChannels, p Params) {
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprint(p.ImageWidth) + "x" + fmt.Sprint(p.ImageHeight) + "x" + fmt.Sprint(p.Turns)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// Wait for the io goroutine to finish writing the image
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
}

func makeCall(client *rpc.Client, initialWorld [][]uint8, turns int, imageWidth, imageHeight int) {
	request := stubs.Request{InitialWorld: initialWorld, Turns: turns, ImageWidth: imageWidth, ImageHeight: imageHeight}
	response := new(stubs.Response)
	client.Call(stubs.ProcessGameOfLifeHandler, request, response)
	fmt.Println("Responded: ")
}

func distributor(p Params, c distributorChannels) {
	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()

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

	if p.Threads == 1 {
		makeCall(client, world, p.Turns, p.ImageWidth, p.ImageHeight)
	}

	var alive []util.Cell
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				alive = append(alive, util.Cell{x, y})
			}
		}
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	output := FinalTurnComplete{p.Turns, alive}
	c.events <- output

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
