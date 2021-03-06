package mpthSim

import (
	"fmt"
	"log"
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
	InputLinks  []*Link
	OutputLinks []*Link
	// TTL is the time to live for a recoder
	Done      chan struct{}
	ResetChan chan struct{}
	// Transmission rate in B/s
	rate    uint64
	Encoder *kodo.Encoder
	Decoder *kodo.Decoder
	Data    []byte

	NodeID    byte
	RxPackets []uint32

	Transmissions uint64

	mu sync.Mutex
}

type payloadWriter interface {
	WritePayload(*uint8) uint32
	PayloadSize() uint32
	Rank() uint32
}

func newNode(rate uint64) *Node {
	n := new(Node)
	n.Done = make(chan struct{})
	n.ResetChan = make(chan struct{})
	n.rate = rate
	return n
}

// NewEncoderNode creates a node with a kodo Encoder. It takes an encoder
// factory as an argument, which it uses to create the encoder
func NewEncoderNode(factory *kodo.EncoderFactory, rate uint64) *Node {
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
func NewDecoderNode(factory *kodo.DecoderFactory, rate uint64) *Node {
	n := newNode(rate)
	n.RxPackets = make([]uint32, 3)
	n.Decoder = factory.Build()
	n.Data = make([]byte, n.Decoder.BlockSize())
	n.Decoder.SetMutableSymbols(&n.Data[0], n.Decoder.BlockSize())
	return n
}

// NewRecoderNode creates a node with a kodo Encoder. It takes an encoder
// factory as an argument, which it uses to create the encoder
func NewRecoderNode(factory *kodo.DecoderFactory, rate uint64) *Node {
	n := newNode(rate)
	n.RxPackets = make([]uint32, 3)
	n.Decoder = factory.Build()
	n.Data = make([]byte, n.Decoder.BlockSize())
	n.Decoder.SetMutableSymbols(&n.Data[0], n.Decoder.BlockSize())
	return n
}

func (n *Node) AddInput(l *Link) {

	n.InputLinks = append(n.InputLinks, l)
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
		n.Inputs = make(chan []byte, 10000)
	}
	n.InputsCount++

	// Start an output goroutine for each new input channel. merger copies
	// values from c to n.Inputs until c is closed, then calls n.InputsWg.Done.
	merger := func(c <-chan []byte) {
		for val := range c {
			n.Inputs <- val
		}
		n.InputsWg.Done()
		n.InputsCount--
	}
	go merger(l.Out)

}

func (n *Node) AddOutput(l *Link) {
	n.OutputLinks = append(n.OutputLinks, l)
}

// SendEncodedPackets produces encoded packets and sends them through all the
// output channels
func (n *Node) SendEncodedPackets() {
	t := float64(n.Encoder.SymbolSize()) / float64(n.rate) * 1000000000 //nS
	debugN("Sending a packet every %v", time.Duration(t)*time.Nanosecond)

	for {
		select {
		case <-n.Done: // The decoder is ready
			for _, output := range n.OutputLinks {
				go close(output.In)
			}
			fmt.Println("Encoder: Got signal done from decoder")
			return
		case <-time.After(time.Duration(t) * time.Nanosecond):
			n.sendPayloads(n.Encoder)
		}
	}
}

func (n *Node) RecodeAndSend() {
	t := float64(n.Decoder.SymbolSize()) / float64(n.rate) * 1000000000 //nS
	fmt.Println("Recoder started")

	// Constantly read packets
	go func() {
		for payload := range n.Inputs {
			n.mu.Lock()
			n.Decoder.ReadPayload(&payload[0])
			n.mu.Unlock()
			// fmt.Println("Recoder rank: ", n.Decoder.Rank())
		}
	}()

	for {
		select {
		case <-n.Done: // The decoder is ready
			for _, output := range n.OutputLinks {
				fmt.Println("Recoder: Got signal done from decoder")
				go close(output.In)
			}
			return
		case <-n.ResetChan: // A reset was triggered
			n.ResetChan = make(chan struct{})
			return
		case <-time.After(time.Duration(t) * time.Nanosecond): // Send a payload
			n.mu.Lock()
			n.sendPayloads(n.Decoder)
			n.mu.Unlock()
		}
	}
}

func (n *Node) ReceiveCodedPackets(wg *sync.WaitGroup, done ...chan<- struct{}) {
	doneIsClosed := false

	for !n.Decoder.IsComplete() {
		for payload := range n.Inputs {
			if doneIsClosed {
				continue
			}
			n.Decoder.ReadPayload(&payload[0])
			id := payload[len(payload)-1]
			n.RxPackets[id]++
			if n.Decoder.IsComplete() {
				// Close all done channels
				for _, d := range done {
					close(d)
				}
				doneIsClosed = true
				log.Println("Decoder is complete!")
				// break
			}
		}
	}
	wg.Done()
}

func (n *Node) Reset(factory *kodo.DecoderFactory) {
	n.mu.Lock()
	defer n.mu.Unlock()
	close(n.ResetChan) // Signal a reset
	fmt.Println("Recoder Reset")
	for _, input := range n.InputLinks {
		input.DestGone = nil
	}

	// Close all current outputs
	for _, output := range n.OutputLinks {
		close(output.In)
	}
	// Reset the Outputs array
	n.OutputLinks = make([]*Link, 0)
	n.InputLinks = make([]*Link, 0)

	defer kodo.DeleteDecoder(n.Decoder) // Delete the recoder

	n.Decoder = factory.Build() // Rebuild the recoder
	n.Data = make([]byte, n.Decoder.BlockSize())
	n.Decoder.SetMutableSymbols(&n.Data[0], n.Decoder.BlockSize())
}

func (n *Node) sendPayloads(coder payloadWriter) {
	if coder.Rank() == 0 {
		return
	}

	tmpOutputs := n.OutputLinks[:0]
	for i, out := range n.OutputLinks {
		if n.OutputLinks[i].DestGone != nil {
			tmpOutputs = append(tmpOutputs, out)
			payload := make([]byte, coder.PayloadSize()+1) // Payload size plus nodeID
			coder.WritePayload(&payload[0])
			payload[len(payload)-1] = n.NodeID // Append the nodeID
			out.In <- payload
			n.Transmissions++
		} else {
			close(out.In)
		}
	}
	n.OutputLinks = tmpOutputs
}
