package mpthSim

import (
	"math/rand"
	"sync"
	"time"

	dbg "github.com/JuanCabre/go-debug"
)

var debugL = dbg.Debug("Link")

// Link represents a communication channel with a loss probability and a delay
type Link struct {
	In       chan []byte
	Out      chan []byte
	lossProb float64
	delay    time.Duration

	DestGone chan struct{}

	InCount, OutCount, LostCount uint64
}

// NewLink creates a new link with the given loss probability and delay
func NewLink(lossProb float64, delay time.Duration) *Link {
	l := new(Link)
	l.In = make(chan []byte, 10000)
	l.Out = make(chan []byte, 10000)
	l.DestGone = make(chan struct{})

	l.delay = delay
	l.lossProb = lossProb

	return l
}

// ProcessPackets listens the Input channel of the link until it is close and
// sends the incoming payload to a go routine DelayAndSend
func (l *Link) ProcessPackets() {

	// WaitGroup to close the output channel of the Link after all packets have
	// been sent
	var wg sync.WaitGroup

	for payload := range l.In {
		debugL("received Packet")
		l.InCount++ // Increase by one the received packets
		// debugL("Received packet: %v", payload)

		// If there are no losses, send the packet
		if rand.Float64() > l.lossProb {
			wg.Add(1)
			go l.delayAndSend(payload, &wg) // Delay and send the packet
		} else {
			l.LostCount++
			debugL("A loss occured")
		}
	}

	// If the Input channel was closed, then we close the out channel after
	// sending all
	wg.Wait()
	debugL("Closing link Channel")
	close(l.Out)
}

// DelayAndSend receives a payload and waits for l.delay before sending it to
// the output channel of the link
func (l *Link) delayAndSend(payload []byte, wg *sync.WaitGroup) {
	<-time.After(l.delay) // Delay the packet
	l.Out <- payload      // Send packet to the output channel
	l.OutCount++          // Increase by one the received packets
	debugL("Sent Packet")
	wg.Done() // Update the information of the waitgroup
}
