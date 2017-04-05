package mpthSim

import (
	"fmt"
	"time"

	"gitlab.com/steinwurf/kodo-go/src/kodo"

	dbg "github.com/JuanCabre/go-debug"
)

var debugN = dbg.Debug("Node")

type Node struct {
	Inputs  []chan []byte
	Outputs []chan<- []byte
	TTL     time.Duration

	Done chan struct{}

	Encoder *kodo.Encoder
	Decoder *kodo.Decoder
}

func (n *Node) AddInput(in chan []byte) {
	n.Inputs = append(n.Inputs, in)
}

func (n *Node) AddOutput(out chan []byte) {
	n.Outputs = append(n.Outputs, out)
}

func (n *Node) GeneratePackets() {
	for {
		select {
		case <-n.Done:
			close(n.Outputs[0])
			return
		case <-time.After(time.Millisecond):
			n.Outputs[0] <- []byte("Hello world")
		}
	}
}

func (n *Node) ConsumePacket(m int, done chan<- struct{}) {
	i := 0
	for packet := range n.Inputs[0] {
		fmt.Println("Destination Received a packet: ", string(packet))
		i++
		if i >= m {
			break
		}
	}
	fmt.Println(m, "packets consumed")
	close(done)
}
