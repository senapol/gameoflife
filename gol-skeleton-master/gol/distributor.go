package gol

import (
	"flag"
	"fmt"
	"log"
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

var brokerAddress = flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")

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
	// Subscribe to a response topic
	responseTopic := "GameOfLifeResponse"
	client.Call("Broker.CreateChannel", stubs.ChannelRequest{Topic: responseTopic, Buffer: 1}, new(stubs.StatusReport))

	golRequest := stubs.GameOfLifeRequest{
		Type:    "StopGameLoop", // Specify the type of Game of Life request
		Request: stubs.AliveCountRequest{},
	}

	// Create a publish request for the broker
	publishReq := stubs.PublishRequest{
		Topic:   "GameOfLife",
		Request: golRequest,
	}

	client.Call("Broker.Publish", publishReq, new(stubs.StatusReport))

	// Wait for response
	responseChan := make(chan interface{})
	go waitForResponse(client, responseTopic, responseChan)

	// Blocking wait for response
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

func pauseServerEvaluation(paused bool, client *rpc.Client) {
	// Subscribe to a response topic
	responseTopic := "GameOfLifeResponse"
	client.Call("Broker.CreateChannel", stubs.ChannelRequest{Topic: responseTopic, Buffer: 1}, new(stubs.StatusReport))

	golRequest := stubs.GameOfLifeRequest{
		Type:    "StopGameLoop", // Specify the type of Game of Life request
		Request: stubs.PauseRequest{Pause: paused},
	}

	// Create a publish request for the broker
	publishReq := stubs.PublishRequest{
		Topic:   "GameOfLife",
		Request: golRequest,
	}

	client.Call("Broker.Publish", publishReq, new(stubs.StatusReport))

	// Wait for response
	responseChan := make(chan interface{})
	go waitForResponse(client, responseTopic, responseChan)

	// Blocking wait for response
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

func stopGameLoop(client *rpc.Client) {
	// Subscribe to a response topic
	statusReport := new(stubs.StatusReport)
	responseTopic := "GameOfLifeResponse"
	fmt.Println("before create channel")
	err := client.Call("Broker.CreateChannel", stubs.ChannelRequest{Topic: responseTopic, Buffer: 1}, &statusReport)
	if err != nil {
		log.Printf("RPC call failed: %v", err)
	}
	fmt.Println("after create channel")

	golRequest := stubs.GameOfLifeRequest{
		Type:    "StopGameLoop", // Specify the type of Game of Life request
		Request: stubs.StopRequest{},
	}

	// Create a publish request for the broker
	publishReq := stubs.PublishRequest{
		Topic:   "GameOfLife",
		Request: golRequest,
	}

	client.Call("Broker.Publish", publishReq, new(stubs.StatusReport))
	fmt.Println("we called the broker")
	// Wait for response
	responseChan := make(chan interface{})
	go waitForResponse(client, responseTopic, responseChan)

	// Blocking wait for response
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

func getAliveCellsCount(client *rpc.Client) (*stubs.AliveCountResponse, error) {
	// Subscribe to a response topic
	responseTopic := "GameOfLifeResponse"
	client.Call("Broker.CreateChannel", stubs.ChannelRequest{Topic: responseTopic, Buffer: 1}, new(stubs.StatusReport))

	golRequest := stubs.GameOfLifeRequest{
		Type:    "StopGameLoop", // Specify the type of Game of Life request
		Request: stubs.AliveCountRequest{},
	}

	// Create a publish request for the broker
	publishReq := stubs.PublishRequest{
		Topic:   "GameOfLife",
		Request: golRequest,
	}

	client.Call("Broker.Publish", publishReq, new(stubs.StatusReport))

	// Wait for response
	responseChan := make(chan interface{})
	go waitForResponse(client, responseTopic, responseChan)

	// Blocking wait for response
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

func makeCall(client *rpc.Client, initialWorld [][]uint8, turns int, imageWidth, imageHeight int) *stubs.Response {
	// Subscribe to a response topic
	responseTopic := "GameOfLifeResponse"
	client.Call("Broker.CreateChannel", stubs.ChannelRequest{Topic: responseTopic, Buffer: 1}, new(stubs.StatusReport))

	golRequest := stubs.GameOfLifeRequest{
		Type: "ProcessGameOfLife", // Specify the type of Game of Life request
		Request: stubs.Request{ // The actual request details
			InitialWorld: initialWorld,
			Turns:        turns,
			ImageWidth:   imageWidth,
			ImageHeight:  imageHeight,
		},
	}

	// Create a publish request for the broker
	publishReq := stubs.PublishRequest{
		Topic:   "GameOfLife",
		Request: golRequest,
	}

	client.Call("Broker.Publish", publishReq, new(stubs.StatusReport))

	// Wait for response
	responseChan := make(chan interface{})
	go waitForResponse(client, responseTopic, responseChan)

	// Blocking wait for response
	res := <-responseChan
	// Process the response
	if resp, ok := res.(*stubs.Response); ok {
		fmt.Println("Responded")
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

func resetServerState(client *rpc.Client, width, height int, world [][]uint8) error {
	// Subscribe to a response topic
	responseTopic := "GameOfLifeResponse"
	client.Call("Broker.CreateChannel", stubs.ChannelRequest{Topic: responseTopic, Buffer: 1}, new(stubs.StatusReport))

	golRequest := stubs.GameOfLifeRequest{
		Type:    "StopGameLoop", // Specify the type of Game of Life request
		Request: stubs.ResetStateRequest{ImageWidth: width, ImageHeight: height, World: world},
	}

	// Create a publish request for the broker
	publishReq := stubs.PublishRequest{
		Topic:   "GameOfLife",
		Request: golRequest,
	}

	client.Call("Broker.Publish", publishReq, new(stubs.StatusReport))

	// Wait for response
	responseChan := make(chan interface{})
	go waitForResponse(client, responseTopic, responseChan)

	// Blocking wait for response
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

func shutDownBroker(client *rpc.Client) {
	shutdownReq := new(stubs.ShutdownRequest)
	shutdownRes := new(stubs.ShutdownResponse)
	err := client.Call("Broker.Shutdown", shutdownReq, shutdownRes)
	if err != nil {
		fmt.Println("Error in RPC call to shut down server:", err)
	}
	fmt.Println(shutdownRes.Message)
}

func shutdownServer(client *rpc.Client) {
	// Subscribe to a response topic
	responseTopic := "GameOfLifeResponse"
	client.Call("Broker.CreateChannel", stubs.ChannelRequest{Topic: responseTopic, Buffer: 1}, new(stubs.StatusReport))

	golRequest := stubs.GameOfLifeRequest{
		Type:    "StopGameLoop", // Specify the type of Game of Life request
		Request: stubs.ShutdownRequest{},
	}

	// Create a publish request for the broker
	publishReq := stubs.PublishRequest{
		Topic:   "GameOfLife",
		Request: golRequest,
	}

	client.Call("Broker.Publish", publishReq, new(stubs.StatusReport))

	// Wait for response
	responseChan := make(chan interface{})
	go waitForResponse(client, responseTopic, responseChan)

	// Blocking wait for response
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

func waitForResponse(client *rpc.Client, topic string, responseChan chan interface{}) {
	for {
		// Implement logic to receive a response from the response topic
		var response stubs.Response
		client.Call("Broker.Subscribe", stubs.Subscription{
			Topic: topic,
			// Callback: This should be a method on the client to handle responses
			Callback: "ClientMethodToHandleResponse",
		}, new(stubs.StatusReport))

		responseChan <- &response
	}

}

func distributor(p Params, c distributorChannels) {
	flag.Parse()
	client, _ := rpc.Dial("tcp", *brokerAddress)
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
	fmt.Println("made it to before stopgameloop")
	//stopping game loop execution
	stopGameLoop(client)

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
					response, err := getAliveCellsCount(client)
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
					quit = true
					shutDown = true
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
			stopGameLoop(client)

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
		stopGameLoop(client)

		//resetting server state
		if err := resetServerState(client, p.ImageWidth, p.ImageHeight, world); err != nil {
			fmt.Println("Error resetting server state:", err)
			return
		}
	} else if shutDown {
		//shut down server and broker
		shutdownServer(client)
		shutDownBroker(client)
	}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(done)
	close(c.events)
}
