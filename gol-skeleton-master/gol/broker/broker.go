package broker

import (
	"errors"
	"fmt"
	"net/rpc"
	"sync"
)

var (
	topics  = make(map[string]chan Message) // Change Message type as per your requirement
	topicMx sync.RWMutex
)

// Message represents a request or response in the message queue
type Message struct {
	Type         string
	Payload      interface{}
	ResponseChan chan<- interface{}
}

// Broker struct to simulate a message broker
type Broker struct {
	queue     chan Message
	wg        sync.WaitGroup
	rpcClient *rpc.Client
}

type RPCRequestMessage struct {
	MethodName   string
	Request      interface{}
	Response     interface{}
	ResponseChan chan<- interface{}
}

// NewBroker creates a new Broker instance
func NewBroker(client *rpc.Client) *Broker {
	return &Broker{
		queue:     make(chan Message, 10),
		rpcClient: client,
	}
}

// Start initiates the message processing
func (b *Broker) Start() {
	go b.processMessages()
}

// Stop waits for all processing to finish
func (b *Broker) Stop() {
	close(b.queue)
	b.wg.Wait()
}

// SendMessage enqueues a message to be processed
func (b *Broker) SendMessage(msg Message) {
	select {
	case b.queue <- msg:
		//message sent
	default:
		// Handle the case when the Broker is too busy
		fmt.Println("Broker is too busy to accept the message")
	}
}

// processMessages handles the messages in the queue
func (b *Broker) processMessages() {
	for msg := range b.queue {
		b.wg.Add(1)
		go b.handleMessage(msg)
	}
}

func (b *Broker) handleMessage(msg Message) {
	defer b.wg.Done()
	switch msg.Type {
	case "RPCRequest":
		rpcMsg := msg.Payload.(RPCRequestMessage)
		// Perform the RPC call
		err := b.rpcClient.Call(rpcMsg.MethodName, rpcMsg.Request, rpcMsg.Response)

		// Send the response or error back through the channel
		if err != nil {
			// Send error back through the channel
			rpcMsg.ResponseChan <- err
		} else {
			// Send the response back through the channel
			rpcMsg.ResponseChan <- rpcMsg.Response
		}

		// Close the response channel after sending the message
		close(rpcMsg.ResponseChan)
	}
}

func publish(topic string, msg Message) error {
	topicMx.RLock()
	defer topicMx.RUnlock()
	if ch, ok := topics[topic]; ok {
		ch <- msg
		return nil
	}
	return errors.New("no such topic")
}

func subscribe(topic string, callback func(Message)) error {
	topicMx.RLock()
	ch, ok := topics[topic]
	topicMx.RUnlock()
	if !ok {
		return errors.New("no such topic")
	}
	go func() {
		for msg := range ch {
			callback(msg)
		}
	}()
	return nil
}
