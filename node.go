package mpthSim

import (
	"fmt"
	"sync"
	"time"

	"gitlab.com/steinwurf/kodo-go/src/kodo"

	dbg "github.com/JuanCabre/go-debug"
)

var debugN = dbg.Debug("Node")

type Node struct {
	// Inputs and Outputs are the channels from which and to which the node
	// receives and sends payloads
	Inputs      chan []byte
	InputsWg    sync.WaitGroup
	InputsCount uint32
	Outputs     []chan []byte
	// TTL is the time to live for a recoder
	TTL  time.Duration
	Done chan struct{}
	// Transmission rate in B/s
	rate    float64
	Encoder *kodo.Encoder
	Decoder *kodo.Decoder
	Data    []byte
}

func newNode(rate float64) *Node {
	n := new(Node)
	n.Done = make(chan struct{})
	n.rate = rate
	return n
}

// NewEncoderNode creates a node with a kodo Encoder. It takes an encoder
// factory as an argument, which it uses to create the encoder
func NewEncoderNode(factory *kodo.EncoderFactory, rate float64) *Node {
	n := newNode(rate)
	n.Encoder = factory.Build()
	n.Data = make([]byte, n.Encoder.BlockSize())
	return n
}

// SetConstSymbols should be called after the n.Data slice have been filled with
// the desired data
func (n *Node) SetConstSymbols() {
	n.Encoder.SetConstSymbols(&n.Data[0], n.Encoder.BlockSize())
}

// NewDecoderNode creates a node with a kodo Encoder. It takes an encoder
// factory as an argument, which it uses to create the encoder
func NewDecoderNode(factory *kodo.DecoderFactory, rate float64) *Node {
	n := newNode(rate)
	n.Decoder = factory.Build()
	n.Data = make([]byte, n.Decoder.BlockSize())
	n.Decoder.SetMutableSymbols(&n.Data[0], n.Decoder.BlockSize())
	return n
}

// NewRecoderNode creates a node with a kodo Encoder. It takes an encoder
// factory as an argument, which it uses to create the encoder
func NewRecoderNode(factory *kodo.DecoderFactory, rate float64) *Node {
	n := newNode(rate)
	n.Decoder = factory.Build()
	n.Data = make([]byte, n.Decoder.BlockSize())
	n.Decoder.SetMutableSymbols(&n.Data[0], n.Decoder.BlockSize())
	return n
}

func (n *Node) AddInput(in chan []byte) {

	n.InputsWg.Add(1)
	// The first time it is called...
	if n.InputsCount == 0 {
		// ...start a goroutine to close n.Inputs once all the output goroutines
		// are done. This must start after the wg.Add call.
		go func() {
			n.InputsWg.Wait()
			close(n.Inputs)
		}()
		// Create the input channel
		n.Inputs = make(chan []byte, 5000)
	}
	n.InputsCount++

	// Start an output goroutine for each new input channel. merger copies
	// values from c to n.Inputs until c is closed, then calls n.InputsWg.Done.
	merger := func(c <-chan []byte) {
		for val := range c {
			n.Inputs <- val
		}
		n.InputsWg.Done()
	}
	go merger(in)

}

func (n *Node) AddOutput(out chan []byte) {
	n.Outputs = append(n.Outputs, out)
}

// SendEncodedPackets produces encoded packets and sends them through all the
// output channels
func (n *Node) SendEncodedPackets() {
	t := float64(n.Encoder.PayloadSize()) / float64(n.rate) * 1000000000 //nS
	debugN("Sending a packet every %v", time.Duration(t)*time.Nanosecond)

	for {

		select {
		case <-n.Done: // The decoder is ready
			for _, output := range n.Outputs {
				close(output)
			}
			return
		case <-time.After(time.Duration(t) * time.Nanosecond):
			for _, output := range n.Outputs {
				payload := make([]byte, n.Encoder.PayloadSize())
				n.Encoder.WritePayload(&payload[0])
				fmt.Printf("%T: &payload=%p | &payload[0]=%p | payload[:]=%v\n", payload, &payload, &payload[0], payload)
				output <- payload
			}
		}
	}
}

func (n *Node) RecodeAndSend() {
	t := float64(n.Decoder.PayloadSize()) / float64(n.rate) * 1000000000 //nS
	debugN("Sending a recoded packet every", time.Duration(t)*time.Nanosecond)

	// Constantly read packets
	go func() {
		for payload := range n.Inputs {
			n.Decoder.ReadPayload(&payload[0])
		}
	}()

	payload := make([]byte, n.Decoder.PayloadSize())
	for {

		select {
		case <-n.Done: // The decoder is ready
			for _, output := range n.Outputs {
				close(output)
			}
			return
		case <-time.After(time.Duration(t) * time.Nanosecond):
			for _, output := range n.Outputs {
				n.Decoder.WritePayload(&payload[0])
				output <- payload
			}
		}
	}
}

func (n *Node) ReceiveCodedPackets(done ...chan<- struct{}) {
	for payload := range n.Inputs {
		n.Decoder.ReadPayload(&payload[0])
		fmt.Println("Decoder rank: ", n.Decoder.Rank())
		if n.Decoder.IsComplete() {
			// Close all done channels
			for _, d := range done {
				close(d)
			}
			return
		}
	}
}
