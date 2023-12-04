package stubs

import "encoding/gob"

var ProcessGameOfLifeHandler = "GameOfLifeOperations.ProcessGameOfLife"

func init() {
	gob.Register(StopRequest{})
	gob.Register(StopResponse{})
	gob.Register(GameOfLifeResponse{})
	// Register other custom types here if necessary
}

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
	// We can add fields here if needed, for example, a message or identifier
}

// StopResponse is the response struct for the stop game loop request.
type StopResponse struct {
	Message string // A message from the server acknowledging the stop request
}

type StatusReport struct {
	Message string // Can be used to relay success or error messages
}

// ChannelRequest is used to request the creation of a new topic channel in the broker.
type ChannelRequest struct {
	Topic  string // The name of the topic
	Buffer int    // Buffer size for the channel
}

// PublishRequest is used by clients to publish data to a topic.
type PublishRequest struct {
	Topic   string            // The topic to which the request is published
	Request GameOfLifeRequest // The actual Game of Life request being sent
}

// Subscription is used to subscribe to a topic on the broker.
type Subscription struct {
	Topic         string // The topic to subscribe to
	ServerAddress string // The address of the server that is subscribing
	Callback      string // The callback method on the server to process the request
}

type GameOfLifeRequest struct {
	Type    string      // Type of request
	Request interface{} // Changed to interface{} to support various request/response types
}

type GameOfLifeResponse struct {
	Type     string      // Type of response
	Response interface{} // Changed to interface{} for flexibility
}
