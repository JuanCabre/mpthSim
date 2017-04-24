package main

import (
	"fmt"
	"math/rand"
	"time"

	"gitlab.com/steinwurf/kodo-go/src/kodo"

	"github.com/JuanCabre/mpthSim"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// The links
	l1 := mpthSim.NewLink(0.4, 100*time.Millisecond)
	go l1.ProcessPackets()
	l2 := mpthSim.NewLink(0.4, 100*time.Millisecond)
	go l2.ProcessPackets()

	// The coders
	// Set the number of symbols (i.e. the generation size in RLNC
	// terminology) and the size of a symbol in bytes
	var symbols, symbolSize uint32 = 10, 100

	// Initialization of encoder and decoder
	encoderFactory := kodo.NewEncoderFactory(kodo.FullVector,
		kodo.Binary8, symbols, symbolSize)
	decoderFactory := kodo.NewDecoderFactory(kodo.FullVector,
		kodo.Binary8, symbols, symbolSize)

	// These lines show the API to clean the memory used by the factories
	defer kodo.DeleteEncoderFactory(encoderFactory)
	defer kodo.DeleteDecoderFactory(decoderFactory)

	encoderNode := mpthSim.NewEncoderNode(encoderFactory, 1000)
	encoderNode.AddOutput(l1.In)
	encoderNode.AddOutput(l2.In)
	// Just for fun - fill the data with random data
	for i := range encoderNode.Data {
		encoderNode.Data[i] = uint8(rand.Uint32())
	}
	encoderNode.SetConstSymbols()

	decoderNode := mpthSim.NewDecoderNode(decoderFactory, 1000)
	decoderNode.AddInput(l1.Out)
	decoderNode.AddInput(l2.Out)

	go encoderNode.SendEncodedPackets()
	decoderNode.ReceiveCodedPackets(encoderNode.Done)

	// for {
	// }

	// Check if we properly decoded the data
	for i, v := range encoderNode.Data {
		if v != decoderNode.Data[i] {
			fmt.Println("Unexpected failure to decode")
			fmt.Println("Please file a bug report :)")
			return
		}
	}
	fmt.Println("Data decoded correctly")
}
