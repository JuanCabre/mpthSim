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
	Inputs  []chan []byte
	Outputs []chan []byte
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
	n.Inputs = append(n.Inputs, in)
}

func (n *Node) AddOutput(out chan []byte) {
	n.Outputs = append(n.Outputs, out)
}

// SendEncodedPackets produces encoded packets and sends them through all the
// output channels
func (n *Node) SendEncodedPackets() {
	t := float64(n.Encoder.PayloadSize()) / float64(n.rate) * 1000000000 //nS
	debugN("Sending a packet every", time.Duration(t)*time.Nanosecond)
	payload := make([]byte, n.Encoder.PayloadSize())

	for {

		select {
		case <-n.Done: // The decoder is ready
			for _, output := range n.Outputs {
				close(output)
			}
			return
		case <-time.After(time.Duration(t) * time.Nanosecond):
			for _, output := range n.Outputs {
				n.Encoder.WritePayload(&payload[0])
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
		for payload := range n.mergeInputs() {
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
	for payload := range n.mergeInputs() {
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

func (n *Node) mergeInputs() chan []byte {
	var wg sync.WaitGroup
	merged := make(chan []byte)

	// Start an output goroutine for each input channel in n.Inputs. merger
	// copies values from c to out until c is closed, then calls wg.Done.
	merger := func(c <-chan []byte) {
		for n := range c {
			merged <- n
		}
		wg.Done()
	}
	wg.Add(len(n.Inputs))

	for _, input := range n.Inputs {
		go merger(input)
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(merged)
	}()

	return merged
}
