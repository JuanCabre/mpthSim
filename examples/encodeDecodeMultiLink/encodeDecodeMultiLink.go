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
	l1 := mpthSim.NewLink(0.5, 100*time.Millisecond)
	go l1.ProcessPackets()
	l2 := mpthSim.NewLink(0, 100*time.Millisecond)
	go l2.ProcessPackets()

	// The coders
	// Set the number of symbols (i.e. the generation size in RLNC
	// terminology) and the size of a symbol in bytes
	var symbols, symbolSize uint32 = 30, 100

	// Initialization of encoder and decoder
	encoderFactory := kodo.NewEncoderFactory(kodo.FullVector,
		kodo.Binary8, symbols, symbolSize)
	decoderFactory := kodo.NewDecoderFactory(kodo.FullVector,
		kodo.Binary8, symbols, symbolSize)

	// These lines show the API to clean the memory used by the factories
	defer kodo.DeleteEncoderFactory(encoderFactory)
	defer kodo.DeleteDecoderFactory(decoderFactory)

	encoderNode := mpthSim.NewEncoderNode(encoderFactory, 1000)
	encoderNode.AddOutput(l1)
	// Just for fun - fill the data with random data
	for i := range encoderNode.Data {
		encoderNode.Data[i] = uint8(rand.Uint32())
	}
	encoderNode.SetConstSymbols()

	decoderNode := mpthSim.NewDecoderNode(decoderFactory, 1000)
	decoderNode.AddInput(l1)

	go encoderNode.SendEncodedPackets()

	go func() {
		<-time.After(5 * time.Second)
		encoderNode.AddOutput(l2)
		decoderNode.AddInput(l2)
	}()

	decoderNode.ReceiveCodedPackets(encoderNode.Done)

	decoderNode.InputsWg.Wait()
	fmt.Println("l1 in : ", l1.InCount, "|| l1 out: ", l1.OutCount, "|| l1 losses: ", l1.LostCount)
	fmt.Println("l2 in : ", l2.InCount, "|| l2 out: ", l2.OutCount, "|| l2 losses: ", l2.LostCount)

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
