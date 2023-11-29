package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
)

var (
	topics  = make(map[string]chan stubs.GameOfLifeRequest)
	topicmx sync.RWMutex
)

var serverAddress = flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")

func createTopic(topic string, buflen int) {
	topicmx.Lock()
	defer topicmx.Unlock()
	if _, ok := topics[topic]; !ok {
		topics[topic] = make(chan stubs.GameOfLifeRequest, buflen)
		fmt.Println("Created channel #", topic)
	}
}

func publish(topic string, request stubs.GameOfLifeRequest) (err error) {
	topicmx.RLock()
	defer topicmx.RUnlock()
	if ch, ok := topics[topic]; ok {
		ch <- request
	} else {
		return fmt.Errorf("No such topic: %s", topic)
	}
	return
}

func subscriberLoop(topic chan stubs.GameOfLifeRequest, server *rpc.Client) {
	for {
		request := <-topic
		response := new(stubs.GameOfLifeResponse)
		err := server.Call("GameOfLifeOperations.ProcessRequest", request, response)
		if err != nil {
			fmt.Println("Error:", err)
			fmt.Println("Closing subscriber thread.")
			topic <- request
			break
		}
	}
}

func subscribe(topic string, serverAddress string) (err error) {
	fmt.Println("Subscription request")
	topicmx.RLock()
	ch := topics[topic]
	topicmx.RUnlock()
	server, err := rpc.Dial("tcp", serverAddress) // Use regular RPC Dial
	if err == nil {
		go subscriberLoop(ch, server)
	} else {
		fmt.Println("Error subscribing:", err)
		return err
	}
	return
}

type Broker struct {
	serverClient *rpc.Client
	shutdownChan chan struct{}
}

func NewBroker() *Broker {
	return &Broker{
		shutdownChan: make(chan struct{}),
	}
}

func (b *Broker) CreateChannel(req stubs.ChannelRequest, res *stubs.StatusReport) (err error) {
	fmt.Println("made it in create channel")
	createTopic(req.Topic, req.Buffer)
	return
}

func (b *Broker) Subscribe(req stubs.Subscription, res *stubs.StatusReport) (err error) {
	err = subscribe(req.Topic, req.ServerAddress)
	if err != nil {
		res.Message = "Error during subscription"
	}
	return err
}

func (b *Broker) Publish(req stubs.PublishRequest, res *stubs.StatusReport) (err error) {
	// Logic to determine if the request should be forwarded to the server
	if req.Topic == "GameOfLife" {
		// Forward the request to the server
		var serverResponse stubs.Response
		err = b.serverClient.Call("GameOfLifeOperations.ProcessGameOfLifeRequest", req.Request, &serverResponse)
		if err != nil {
			fmt.Println("Error forwarding request to server:", err)
			return err
		}
		// Handle serverResponse if needed
	} else {
		// Existing publish logic for other topics
		err = publish(req.Topic, req.Request)
	}
	return err
}

func (b *Broker) Shutdown(req *stubs.ShutdownRequest, res *stubs.ShutdownResponse) error {
	close(b.shutdownChan)
	// Respond with success
	res.Success = true
	res.Message = "Server shutting down"
	return nil
}

func (b *Broker) CallProcessGameOfLifeRequest(req stubs.Request, res *stubs.Response) error {
	// Create an RPC client connection to the server
	server, err := rpc.Dial("tcp", *serverAddress)
	if err != nil {
		return err
	}
	defer server.Close()

	// Make an RPC call to the server's ProcessGameOfLifeRequest function
	err = server.Call("GameOfLifeOperations.ProcessGameOfLifeRequest", req, res)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	var err error
	broker := new(Broker)
	broker.serverClient, err = rpc.Dial("tcp", *serverAddress)
	if err != nil {
		panic(err)
	}

	rpc.Register(broker)
	newBroker := NewBroker()
	rpc.Register(newBroker)
	listener, err := net.Listen("tcp", ":8040") // Replace with broker's address
	if err != nil {
		panic(err)
	}
	fmt.Println("everythhing set up, listening")
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-newBroker.shutdownChan:
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
	<-newBroker.shutdownChan
}
