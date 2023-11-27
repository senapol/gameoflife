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
type AliveCountRequest struct{}

type AliveCountResponse struct {
	CompletedTurns int
	Count          int
}

//to pause
type PauseRequest struct {
	Pause bool
}

type PauseResponse struct {
	// Any response fields if needed
}

//to shut down
type ShutdownRequest struct {
	// We can add fields here if needed
}

type ShutdownResponse struct {
	// Fields to indicate the result of the shutdown operation
	Success bool
	Message string
}

//resetting the state:
type ResetStateRequest struct {
	World [][]uint8
}

type ResetStateResponse struct {
	// any response fields we want
}
