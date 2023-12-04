package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/gol/broker"
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

var cAddress = flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")

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

func updateCurrentTurn(b *broker.Broker) int {
	turnRequest := stubs.AliveCountRequest{}
	turnResponse := new(stubs.AliveCountResponse)
	responseChan := make(chan interface{})

	// Create an RPC request message
	rpcRequestMsg := broker.Message{
		Type: "RPCRequest",
		Payload: broker.RPCRequestMessage{
			MethodName:   "GameOfLifeOperations.GetAliveCellsCount",
			Request:      turnRequest,
			Response:     turnResponse,
			ResponseChan: responseChan,
		},
	}

	// Send the message to the Broker
	b.SendMessage(rpcRequestMsg)

	// Wait for the response
	response := <-responseChan

	// Process the response
	if resp, ok := response.(*stubs.AliveCountResponse); ok {
		return resp.CompletedTurns
	} else if err, ok := response.(error); ok {
		fmt.Println("Error in RPC call:", err)
		return -1 // or any other indication of error
	} else {
		fmt.Println("Unexpected response type")
		return -1 // or any other indication of error
	}
}

func pauseServerEvaluation(paused bool, b *broker.Broker) {
	pauseReq := stubs.PauseRequest{Pause: paused}
	pauseRes := new(stubs.PauseResponse)
	responseChan := make(chan interface{})

	// Create an RPC request message
	rpcRequestMsg := broker.Message{
		Type: "RPCRequest",
		Payload: broker.RPCRequestMessage{
			MethodName:   "GameOfLifeOperations.TogglePause",
			Request:      pauseReq,
			Response:     pauseRes,
			ResponseChan: responseChan,
		},
	}

	// Send the message to the Broker
	b.SendMessage(rpcRequestMsg)

	// Wait for the response
	response := <-responseChan

	// Process the response
	if _, ok := response.(*stubs.PauseResponse); ok {
		// Pause toggled successfully
	} else if err, ok := response.(error); ok {
		fmt.Println("Error in RPC call to toggle pause:", err)
	} else {
		fmt.Println("Unexpected response type")
	}
}

func shutdownServer(b *broker.Broker) {
	shutdownReq := new(stubs.ShutdownRequest)
	shutdownRes := new(stubs.ShutdownResponse)
	responseChan := make(chan interface{})

	// Create an RPC request message
	rpcRequestMsg := broker.Message{
		Type: "RPCRequest",
		Payload: broker.RPCRequestMessage{
			MethodName:   "GameOfLifeOperations.Shutdown",
			Request:      shutdownReq,
			Response:     shutdownRes,
			ResponseChan: responseChan,
		},
	}

	// Send the message to the Broker
	b.SendMessage(rpcRequestMsg)

	// Wait for the response
	response := <-responseChan

	// Process the response
	if res, ok := response.(*stubs.ShutdownResponse); ok {
		fmt.Println(res.Message)
	} else if err, ok := response.(error); ok {
		fmt.Println("Error in RPC call to shut down server:", err)
	} else {
		fmt.Println("Unexpected response type")
	}
}

func makeCall(b *broker.Broker, initialWorld [][]uint8, turns int, imageWidth, imageHeight int) *stubs.Response {
	request := stubs.Request{InitialWorld: initialWorld, Turns: turns, ImageWidth: imageWidth, ImageHeight: imageHeight}
	response := new(stubs.Response)
	responseChan := make(chan interface{})

	// Create an RPC request message
	rpcRequestMsg := broker.Message{
		Type: "RPCRequest",
		Payload: broker.RPCRequestMessage{
			MethodName:   stubs.ProcessGameOfLifeHandler,
			Request:      request,
			Response:     response,
			ResponseChan: responseChan,
		},
	}

	// Send the message to the Broker
	b.SendMessage(rpcRequestMsg)

	// Wait for the response
	res := <-responseChan

	// Process the response
	if resp, ok := res.(*stubs.Response); ok {
		fmt.Println("Server Responded")
		return resp
	} else if err, ok := res.(error); ok {
		// Handle the error
		fmt.Println("Error in RPC call:", err)
		return nil
	} else {
		// Handle unexpected response type
		fmt.Println("Unexpected response type")
		return nil
	}
}

func resetServerState(b *broker.Broker, width, height int, world [][]uint8) error {
	req := stubs.ResetStateRequest{ImageWidth: width, ImageHeight: height, World: world}
	res := new(stubs.ResetStateResponse)
	responseChan := make(chan interface{})

	// Create an RPC request message
	rpcRequestMsg := broker.Message{
		Type: "RPCRequest",
		Payload: broker.RPCRequestMessage{
			MethodName:   "GameOfLifeOperations.ResetState",
			Request:      &req,
			Response:     res,
			ResponseChan: responseChan,
		},
	}

	// Send the message to the Broker
	b.SendMessage(rpcRequestMsg)

	// Wait for the response
	response := <-responseChan

	// Check and process the response
	if _, ok := response.(*stubs.ResetStateResponse); ok {
		return nil // No error, successful reset
	} else if err, ok := response.(error); ok {
		return fmt.Errorf("error resetting server state: %v", err)
	} else {
		return fmt.Errorf("unexpected response type")
	}
}

func stopGameLoop(b *broker.Broker) {
	stopReq := new(stubs.StopRequest)
	stopRes := new(stubs.StopResponse)
	responseChan := make(chan interface{})

	// Create an RPC request message
	rpcRequestMsg := broker.Message{
		Type: "RPCRequest",
		Payload: broker.RPCRequestMessage{
			MethodName:   "GameOfLifeOperations.StopGameLoop",
			Request:      stopReq,
			Response:     stopRes,
			ResponseChan: responseChan,
		},
	}

	// Send the message to the Broker
	b.SendMessage(rpcRequestMsg)

	// Wait for the response
	response := <-responseChan

	// Process the response
	if res, ok := response.(*stubs.StopResponse); ok {
		fmt.Println(res.Message)
	} else if err, ok := response.(error); ok {
		// If the response is an error
		fmt.Println("Error stopping game loop:", err)
	} else {
		// Handle unexpected response type
		fmt.Println("Unexpected response type")
	}
}

func getAliveCellsCount(b *broker.Broker) (*stubs.AliveCountResponse, error) {
	request := stubs.AliveCountRequest{}
	response := new(stubs.AliveCountResponse)
	responseChan := make(chan interface{})

	// Create an RPC request message
	rpcRequestMsg := broker.Message{
		Type: "RPCRequest",
		Payload: broker.RPCRequestMessage{
			MethodName:   "GameOfLifeOperations.GetAliveCellsCount",
			Request:      request,
			Response:     response,
			ResponseChan: responseChan,
		},
	}

	// Send the message to the Broker
	b.SendMessage(rpcRequestMsg)

	// Wait for the response
	res := <-responseChan

	// Process the response
	if resp, ok := res.(*stubs.AliveCountResponse); ok {
		return resp, nil
	} else if err, ok := res.(error); ok {
		return nil, fmt.Errorf("error in RPC call: %v", err)
	} else {
		return nil, fmt.Errorf("unexpected response type")
	}
}

func distributor(p Params, c distributorChannels) {
	flag.Parse()
	client, err := rpc.Dial("tcp", *cAddress)
	if err != nil {
		fmt.Println("Client: Failed to connect, retrying...")
		time.Sleep(1 * time.Second) // Retry every second
		for {
			client, err = rpc.Dial("tcp", *cAddress)
			if err != nil {
				fmt.Println("Client: Failed to connect, retrying...")
				time.Sleep(1 * time.Second) // Retry every 5 seconds
			} else {
				fmt.Println("Client: Connected.")
				break
			}
		}
	}
	defer client.Close()
	broker := broker.NewBroker(client)
	broker.Start()
	defer broker.Stop()

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
	stopGameLoop(broker)

	//resetting server state
	if err := resetServerState(broker, p.ImageWidth, p.ImageHeight, world); err != nil {
		//fmt.Println("Error resetting server state:", err)
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
					response, err := getAliveCellsCount(broker)
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
				currentTurn = updateCurrentTurn(broker)
				switch key {
				case 's':
					// Save current state as PGM file
					saveWorldToPGM(world, c, p, currentTurn)
				case 'q':
					quit = true
				case 'k':
					saveWorldToPGM(world, c, p, currentTurn)
					quit = true
					shutDown = true
				case 'p':
					paused = !paused
					pauseServerEvaluation(paused, broker)
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
			stopGameLoop(broker)

			//resetting server state
			if err := resetServerState(broker, p.ImageWidth, p.ImageHeight, world); err != nil {
				//fmt.Println("Error resetting server state:", err)
				return
			}
			return
		}
	}()
	response := makeCall(broker, world, p.Turns, p.ImageWidth, p.ImageHeight)
	world = response.UpdatedWorld
	if !quit && !shutDown {
		//output pgm file
		currentTurn = updateCurrentTurn(broker)
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
		stopGameLoop(broker)

		//resetting server state
		if err := resetServerState(broker, p.ImageWidth, p.ImageHeight, world); err != nil {
			//fmt.Println("Error resetting server state:", err)
			return
		}
	} else if shutDown {
		//shut down server
		shutdownServer(broker)
	}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(done)
	close(c.events)
}
