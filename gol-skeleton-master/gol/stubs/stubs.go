package stubs

var ProcessGameOfLifeHandler = "GameOfLifeOperations.ProcessGameOfLife"

//basic response and requests for the gol logic
type Response struct {
	UpdatedWorld [][]uint8
}

type Request struct {
	InitialWorld [][]uint8
	Turns        int
	ImageWidth   int
	ImageHeight  int
}

//to output alive cells
type AliveCountRequest struct {
	World [][]uint8
}

type AliveCountResponse struct {
	CompletedTurns int
	Count          int
}
