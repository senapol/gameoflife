package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"time"
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

var server = flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")

func saveWorldToPGM(world [][]uint8, c distributorChannels, p Params, currentTurn int) {
	if world == nil || len(world) == 0 {
		fmt.Println("World is empty, skipping PGM save")
		return
	}
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprint(p.ImageWidth) + "x" + fmt.Sprint(p.ImageHeight) + "x" + fmt.Sprint(currentTurn)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// Wait for the io goroutine to finish writing the image
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
}

func updateCurrentTurn(client *rpc.Client) int {
	turnRequest := stubs.AliveCountRequest{}
	turnResponse := new(stubs.AliveCountResponse)
	turnErr := client.Call("GameOfLifeOperations.GetAliveCellsCount", turnRequest, turnResponse)
	if turnErr != nil {
		fmt.Println("Error in RPC call:", turnErr)
	}
	return turnResponse.CompletedTurns
}

func pauseServerEvaluation(paused bool, client *rpc.Client) {
	pauseReq := stubs.PauseRequest{Pause: paused}
	pauseRes := new(stubs.PauseResponse)
	err := client.Call("GameOfLifeOperations.TogglePause", pauseReq, pauseRes)
	if err != nil {
		fmt.Println("Error in RPC call to toggle pause:", err)
	}
}

func makeCall(client *rpc.Client, initialWorld [][]uint8, turns int, imageWidth, imageHeight int) *stubs.Response {
	request := stubs.Request{InitialWorld: initialWorld, Turns: turns, ImageWidth: imageWidth, ImageHeight: imageHeight}
	response := new(stubs.Response)
	client.Call(stubs.ProcessGameOfLifeHandler, request, response)
	fmt.Println("Responded")
	return response
}

func resetServerState(client *rpc.Client, width, height int, world [][]uint8) error {
	req := stubs.ResetStateRequest{ImageWidth: width, ImageHeight: height, World: world}
	res := new(stubs.ResetStateResponse)
	return client.Call("GameOfLifeOperations.ResetState", &req, res)
}

func distributor(p Params, c distributorChannels) {
	flag.Parse()
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()

	paused := false
	//pauseServerEvaluation(paused, client)

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

	//stopping game loop execution
	stopReq := new(stubs.StopRequest)
	stopRes := new(stubs.StopResponse)
	err := client.Call("GameOfLifeOperations.StopGameLoop", stopReq, stopRes)
	if err != nil {
		fmt.Println("Error stopping game loop:", err)
	} else {
		fmt.Println(stopRes.Message)
	}

	//resetting server state
	if err := resetServerState(client, p.ImageWidth, p.ImageHeight, world); err != nil {
		fmt.Println("Error resetting server state:", err)
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	done := make(chan struct{})
	tickerPaused := false
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				if !tickerPaused {
					request := stubs.AliveCountRequest{}
					response := new(stubs.AliveCountResponse)
					err := client.Call("GameOfLifeOperations.GetAliveCellsCount", request, response)
					if err != nil {
						fmt.Println("Error in RPC call:", err)
						continue
					}

					// Send AliveCellsCount event
					c.events <- AliveCellsCount{CompletedTurns: response.CompletedTurns, CellsCount: response.Count}
				}
			case <-done:
				ticker.Stop()
				return
			}

		}
	}()

	currentTurn := 0
	quit := false
	shutDown := false

	go func() {
		for !quit {
			select {
			case key := <-c.keyPresses:
				currentTurn = updateCurrentTurn(client)
				switch key {
				case 's':
					// Save current state as PGM file
					saveWorldToPGM(world, c, p, currentTurn)
				case 'q':
					quit = true
				case 'k':
					saveWorldToPGM(world, c, p, currentTurn)
					shutDown = true

					//shut down
					shutdownReq := new(stubs.ShutdownRequest)
					shutdownRes := new(stubs.ShutdownResponse)
					err := client.Call("GameOfLifeOperations.Shutdown", shutdownReq, shutdownRes)
					if err != nil {
						fmt.Println("Error in RPC call to shut down server:", err)
					}
					fmt.Println(shutdownRes.Message)
				case 'p':
					paused = !paused
					pauseServerEvaluation(paused, client)
					if paused {
						c.events <- StateChange{currentTurn, 0}
						tickerPaused = true
					} else {
						c.events <- StateChange{currentTurn, 1}
						tickerPaused = false
					}
				}
			}
		}
		if quit {
			//paused = true
			//pauseServerEvaluation(paused, client)

			//stopping game loop execution
			stopReq := new(stubs.StopRequest)
			stopRes := new(stubs.StopResponse)
			err := client.Call("GameOfLifeOperations.StopGameLoop", stopReq, stopRes)
			if err != nil {
				fmt.Println("Error stopping game loop:", err)
			} else {
				fmt.Println(stopRes.Message)
			}

			//resetting server state
			if err := resetServerState(client, p.ImageWidth, p.ImageHeight, world); err != nil {
				fmt.Println("Error resetting server state:", err)
				return
			}
			return
		}
	}()
	response := makeCall(client, world, p.Turns, p.ImageWidth, p.ImageHeight)
	world = response.UpdatedWorld
	if !quit && !shutDown {
		//output pgm file
		currentTurn = updateCurrentTurn(client)
		saveWorldToPGM(world, c, p, currentTurn)
		aliveCount := 0
		var alive []util.Cell
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				if world[y][x] == 255 {
					alive = append(alive, util.Cell{x, y})
					aliveCount++
				}
			}
		}

		// TODO: Report the final state using FinalTurnCompleteEvent.
		output := FinalTurnComplete{currentTurn, alive}
		c.events <- output
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{currentTurn, Quitting}
	if !shutDown {
		//stopping game loop execution
		stopReq := new(stubs.StopRequest)
		stopRes := new(stubs.StopResponse)
		err := client.Call("GameOfLifeOperations.StopGameLoop", stopReq, stopRes)
		if err != nil {
			fmt.Println("Error stopping game loop:", err)
		} else {
			fmt.Println(stopRes.Message)
		}

		//resetting server state
		if err := resetServerState(client, p.ImageWidth, p.ImageHeight, world); err != nil {
			fmt.Println("Error resetting server state:", err)
			return
		}
	}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(done)
	close(c.events)
}
