package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
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

/** Super-Secret `reversing a string' method we can't allow clients to see. **/
func updateWorld(startY, endY int, world, worldUpdate [][]uint8, imageHeight, imageWidth int) [][]uint8 {
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

type GameOfLifeOperations struct{}

func (s *GameOfLifeOperations) ProcessGameOfLife(req stubs.Request, res *stubs.Response) (err error) {
	//check that the world isn't empty
	if len(req.InitialWorld) == 0 {
		fmt.Println("Empty World")
		return
	}
	fmt.Println("Got Initial World: ")

	//set up a world update to not edit the original
	worldUpdate := make([][]uint8, req.ImageHeight)
	for i := range req.InitialWorld {
		worldUpdate[i] = make([]uint8, req.ImageWidth)
	}
	for y := 0; y < req.ImageHeight; y++ {
		for x := 0; x < req.ImageWidth; x++ {
			req.InitialWorld[y][x] = req.InitialWorld[y][x]
		}
	}

	//start updating world
	currentTurn := 0
	quit := false

	for currentTurn < req.Turns && !quit {
		worldUpdate = updateWorld(0, req.ImageHeight, req.InitialWorld, worldUpdate, req.ImageWidth, req.ImageHeight)
	}
	res.UpdatedWorld = worldUpdate
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&GameOfLifeOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()

	rpc.Accept(listener)
}
