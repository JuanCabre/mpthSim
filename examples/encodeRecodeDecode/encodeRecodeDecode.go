package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"gitlab.com/steinwurf/kodo-go/src/kodo"

	"github.com/JuanCabre/mpthSim"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup

	// The links
	l1 := mpthSim.NewLink(0.4, 100*time.Millisecond)
	wg.Add(1)
	go Wrapper(l1.ProcessPackets, "l1.ProcessPacket", &wg)
	l2 := mpthSim.NewLink(0.4, 100*time.Millisecond)
	wg.Add(1)
	go Wrapper(l2.ProcessPackets, "l2.ProcessPacket", &wg)

	// The coders
	// Set the number of symbols (i.e. the generation size in RLNC
	// terminology) and the size of a symbol in bytes
	var symbols, symbolSize uint32 = 100, 1000
	var rate float64 = 10000

	// Initialization of encoder and decoder
	encoderFactory := kodo.NewEncoderFactory(kodo.FullVector,
		kodo.Binary8, symbols, symbolSize)
	decoderFactory := kodo.NewDecoderFactory(kodo.FullVector,
		kodo.Binary8, symbols, symbolSize)

	// These lines show the API to clean the memory used by the factories
	defer kodo.DeleteEncoderFactory(encoderFactory)
	defer kodo.DeleteDecoderFactory(decoderFactory)

	encoderNode := mpthSim.NewEncoderNode(encoderFactory, rate)
	encoderNode.AddOutput(l1.In)
	// Just for fun - fill the data with random data
	for i := range encoderNode.Data {
		encoderNode.Data[i] = uint8(rand.Uint32())
	}
	encoderNode.SetConstSymbols()

	recoderNode := mpthSim.NewRecoderNode(decoderFactory, rate)
	recoderNode.AddInput(l1.Out)
	recoderNode.AddOutput(l2.In)

	decoderNode := mpthSim.NewDecoderNode(decoderFactory, rate)
	decoderNode.AddInput(l2.Out)

	wg.Add(2)
	go Wrapper(encoderNode.SendEncodedPackets, "encoderNode.SendEncodedPackets", &wg)
	go Wrapper(recoderNode.RecodeAndSend, "recoderNode.RecodeAndSend", &wg)

	decoderNode.ReceiveCodedPackets(recoderNode.Done, encoderNode.Done)

	// Check if we properly decoded the data
	for i, v := range encoderNode.Data {
		if v != decoderNode.Data[i] {
			fmt.Println("Unexpected failure to decode")
			fmt.Println("Please file a bug report :)")
			return
		}
	}
	fmt.Println("Data decoded correctly")
	wg.Wait()
}

func Wrapper(f func(), name string, wg *sync.WaitGroup) {
	f()
	fmt.Println(name, "returned")
	wg.Done()
}
