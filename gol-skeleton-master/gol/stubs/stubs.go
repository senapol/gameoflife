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
	ImageHeight int
	ImageWidth  int
	World       [][]uint8
}

type ResetStateResponse struct {
	// any response fields we want
}

//stopping game loop if client disconnects
type StopRequest struct {
	// You can add fields here if needed, for example, a message or identifier
}

// StopResponse is the response struct for the stop game loop request.
type StopResponse struct {
	Message string // A message from the server acknowledging the stop request
}
