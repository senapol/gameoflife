package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
)

var (
	gameOfLifeOps *GameOfLifeOperations
	brokerClient  *rpc.Client
)

var brokerAddress = flag.String("server", "127.0.0.1:8040", "IP:port string to connect to as server")

func getOutboundIP() string {
	conn, _ := net.Dial("udp", "8.8.8.8:80")
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr).IP.String()
	return localAddr
}

func countNeighbours(y, x int, world [][]uint8, imageHeight, imageWidth int) int {
	neighbours := 0
	for y1 := -1; y1 <= 1; y1++ {
		for x1 := -1; x1 <= 1; x1++ {
			if world[(y+y1+imageHeight)%imageHeight][(x+x1+imageWidth)%imageWidth] == 255 && (y1 != 0 || x1 != 0) {
				neighbours++
			}
		}
	}
	return neighbours
}

func (s *GameOfLifeOperations) GetAliveCellsCount(req stubs.AliveCountRequest, res *stubs.AliveCountResponse) error {
	// Calculate the number of alive cells in the current world state
	s.mutex.Lock()
	aliveCount := countAliveCells(s.currentWorld)
	s.mutex.Unlock()
	res.CompletedTurns = s.currentTurn
	res.Count = aliveCount
	return nil
}

func (s *GameOfLifeOperations) Shutdown(req stubs.ShutdownRequest, res *stubs.ShutdownResponse) error {
	close(s.shutdownChan)
	// Respond with success
	res.Success = true
	res.Message = "Server shutting down"
	return nil
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

func updateWorld(startY, endY int, world, worldUpdate [][]uint8, imageHeight, imageWidth int) [][]uint8 {
	//fmt.Println("image height ", imageHeight, "image width ", imageWidth, "in function world heght ", len(world), "in function world width ", len(world[0]))
	for y := startY; y < endY; y++ {
		for x := 0; x < imageWidth; x++ {
			neighbours := countNeighbours(y, x, world, imageHeight, imageWidth)

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

type GameOfLifeOperations struct {
	currentTurn     int
	paused          bool
	currentWorld    [][]uint8
	mutex           sync.Mutex
	shutdownChan    chan struct{}
	maintainState   bool
	imageHeight     int
	imageWidth      int
	shouldStop      bool
	clientConnected bool
}

func (s *GameOfLifeOperations) ResetState(req stubs.ResetStateRequest, res *stubs.ResetStateResponse) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.imageWidth = req.ImageWidth
	s.imageHeight = req.ImageHeight
	s.shouldStop = false
	s.paused = false
	s.currentTurn = 0
	s.currentWorld = make([][]uint8, len(req.World))
	for i := range req.World {
		s.currentWorld[i] = make([]uint8, len(req.World[i]))
	}
	fmt.Println("Server state reset")
	for y := 0; y < len(req.World); y++ {
		for x := 0; x < len(req.World[y]); x++ {
			s.currentWorld[y][x] = req.World[y][x]
		}
	}

	return nil
}

func (s *GameOfLifeOperations) StopGameLoop(req stubs.StopRequest, res *stubs.StopResponse) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.shouldStop = true
	res.Message = "Game loop will be stopped"
	return nil
}

func NewGameOfLifeOperations() *GameOfLifeOperations {
	return &GameOfLifeOperations{
		shutdownChan:    make(chan struct{}),
		mutex:           sync.Mutex{},
		currentTurn:     0,
		shouldStop:      false,
		clientConnected: true,
	}
}

func (s *GameOfLifeOperations) TogglePause(req stubs.PauseRequest, res *stubs.PauseResponse) error {
	s.paused = req.Pause
	return nil
}

func (s *GameOfLifeOperations) ProcessGameOfLife(req stubs.Request, res *stubs.Response) (err error) {
	//fmt.Println("image height ", s.imageHeight, "image width ", s.imageWidth, "given world heght ", len(req.InitialWorld), "given world width ", len(req.InitialWorld[0]))
	//check that the world isn't empty
	if len(req.InitialWorld) == 0 {
		fmt.Println("Empty World")
		return
	}
	fmt.Println("Got Initial World")
	s.maintainState = true
	//fmt.Println("image height ", s.imageHeight, "image width ", s.imageWidth, "local world heght ", len(s.currentWorld), "local world width ", len(s.currentWorld[0]))

	//set up a world update to not edit the original
	worldUpdate := make([][]uint8, s.imageHeight)
	for i := range worldUpdate {
		worldUpdate[i] = make([]uint8, s.imageWidth)
	}
	//fmt.Println("image height ", s.imageHeight, "image width ", s.imageWidth, "update world heght ", len(worldUpdate), "update world width ", len(worldUpdate[0]))

	for y := 0; y < s.imageHeight; y++ {
		for x := 0; x < s.imageWidth; x++ {
			worldUpdate[y][x] = s.currentWorld[y][x]
		}
	}

	//start updating world
	quit := false
	for s.currentTurn < req.Turns && !quit {
		if s.shouldStop || !s.clientConnected {
			break
		}
		if !s.paused {
			s.mutex.Lock()
			worldUpdate = updateWorld(0, s.imageHeight, s.currentWorld, worldUpdate, s.imageWidth, s.imageHeight)
			s.currentTurn++
			fmt.Println("current turn is ", s.currentTurn)
			for y := 0; y < s.imageHeight; y++ {
				for x := 0; x < s.imageWidth; x++ {
					s.currentWorld[y][x] = worldUpdate[y][x]
				}
			}
			s.mutex.Unlock()
		}
	}
	res.UpdatedWorld = worldUpdate
	return
}

func subscribeToBroker(brokerClient *rpc.Client, topic, callbackMethod string) {
	serverIP := getOutboundIP()
	serverAddress := fmt.Sprintf("%s:%s", serverIP, "8030")

	subscription := stubs.Subscription{
		Topic:         topic,
		ServerAddress: serverAddress,
		Callback:      callbackMethod,
	}

	var status stubs.StatusReport
	err := brokerClient.Call("Broker.Subscribe", subscription, &status)
	if err != nil {
		fmt.Println("not subbed")
		log.Fatalf("Error subscribing to topic: %v", err)
	}
}

func (s *GameOfLifeOperations) ProcessGameOfLifeRequest(req stubs.GameOfLifeRequest, res *stubs.Response) error {
	// Based on the request type, call the appropriate method
	var err error
	fmt.Println("in here, type ", req.Type)
	switch req.Type {
	case "GetAliveCellsCount":
		var res stubs.AliveCountResponse
		err = s.GetAliveCellsCount(stubs.AliveCountRequest{}, &res)
		if err != nil {
			return err
		}
		// Publish response back to broker
		publishResponseToBroker("GameOfLife", "GameOfLife", res)

	case "ProcessGameOfLife":
		// Use type assertion to convert actualRequest to the expected type
		actualRequest, ok := req.Request.(stubs.Request)
		if !ok {
			return fmt.Errorf("invalid request type for ProcessGameOfLife")
		}
		var res stubs.Response
		err = s.ProcessGameOfLife(actualRequest, &res)
		if err != nil {
			return err
		}
		// Publish response back to broker
		publishResponseToBroker("GameOfLife", "GameOfLife", res)

	case "TogglePause":
		actualRequest, ok := req.Request.(stubs.PauseRequest)
		if !ok {
			return fmt.Errorf("invalid request type for ProcessGameOfLife")
		}
		var res stubs.PauseResponse
		err = s.TogglePause(actualRequest, &res)
		if err != nil {
			return err
		}
		// Publish response back to broker
		publishResponseToBroker("GameOfLife", "GameOfLife", res)
	case "Shutdown":
		var res stubs.ShutdownResponse
		err = s.Shutdown(stubs.ShutdownRequest{}, &res)
		if err != nil {
			return err
		}
		// Publish response back to broker
		publishResponseToBroker("GameOfLife", "GameOfLife", res)
	case "ResetState":
		actualRequest, ok := req.Request.(stubs.ResetStateRequest)
		if !ok {
			return fmt.Errorf("invalid request type for ProcessGameOfLife")
		}
		var res stubs.ResetStateResponse
		err = s.ResetState(actualRequest, &res)
		if err != nil {
			return err
		}
		// Publish response back to broker
		publishResponseToBroker("GameOfLife", "GameOfLife", res)
	case "StopGameLoop":
		var res stubs.StopResponse
		err = s.StopGameLoop(stubs.StopRequest{}, &res)
		if err != nil {
			return err
		}
		// Publish response back to broker
		publishResponseToBroker("GameOfLife", "GameOfLife", res)

	default:
		return fmt.Errorf("unknown request type: %s", req.Type)
	}

	return nil
}

func publishResponseToBroker(topic string, responseType string, response interface{}) {
	// Wrap the response in GameOfLifeResponse
	golResponse := stubs.GameOfLifeResponse{
		Type:     responseType,
		Response: response,
	}

	// Perform an RPC call to the broker's Publish method
	var status stubs.StatusReport
	err := brokerClient.Call("Broker.HandleResponse", golResponse, &status)
	if err != nil {
		fmt.Printf("Failed to send response to broker for topic '%s': %v\n", topic, err)
	} else {
		fmt.Printf("Response for topic '%s' sent to broker: %s\n", topic, status.Message)
	}
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	gameOfLifeOps := NewGameOfLifeOperations()
	rpc.Register(gameOfLifeOps)
	//rpc.Register(&GameOfLifeOperations{})

	go func() {
		var clientErr error
		for {
			brokerClient, clientErr = rpc.Dial("tcp", *brokerAddress)
			if clientErr != nil {
				fmt.Println("Server: Failed to connect to broker, retrying...")
				time.Sleep(1 * time.Second) // Retry every 5 seconds
			} else {
				fmt.Println("Server: Connected to broker.")
				// Perform any additional setup if needed
				subscribeToBroker(brokerClient, "GameOfLife", "ProcessGameOfLifeRequest")
				break
			}
		}
	}()

	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println("Error listening: ", err.Error())
		return
	}
	defer listener.Close()
	defer brokerClient.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-gameOfLifeOps.shutdownChan:
					fmt.Println("Shutting down the server")
					return // Exit the loop on shutdown
				default:
					fmt.Println("Accept error:", err)
					continue
				}
			}
			go rpc.ServeConn(conn)
		}
	}()

	// Wait for shutdown signal
	<-gameOfLifeOps.shutdownChan
}
