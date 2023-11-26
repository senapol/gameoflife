package stubs

var ProcessGameOfLifeHandler = "GameOfLifeOperations.ProcessGameOfLife"

type Response struct {
	UpdatedWorld [][]uint8
}

type Request struct {
	InitialWorld [][]uint8
	Turns        int
	ImageWidth   int
	ImageHeight  int
}

type AliveCountRequest struct {
	World [][]uint8
}

type AliveCountResponse struct {
	CompletedTurns int
	Count          int
}
