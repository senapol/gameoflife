package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
)

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

func (s *GameOfLifeOperations) Shutdown(req *stubs.ShutdownRequest, res *stubs.ShutdownResponse) error {
	// Implement the shutdown logic here
	// For example, you might want to set a flag that causes the server loop to exit
	// or perform some cleanup operations
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

func (s *GameOfLifeOperations) StopGameLoop(req *stubs.StopRequest, res *stubs.StopResponse) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.shouldStop = true
	res.Message = "Game loop will be stopped"
	return nil
}

func NewGameOfLifeOperations() *GameOfLifeOperations {
	return &GameOfLifeOperations{
		shutdownChan:    make(chan struct{}),
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

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	gameOfLifeOps := NewGameOfLifeOperations()
	rpc.Register(gameOfLifeOps)
	rpc.Register(&GameOfLifeOperations{})

	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println("Error listening: ", err.Error())
		return
	}
	defer listener.Close()

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
