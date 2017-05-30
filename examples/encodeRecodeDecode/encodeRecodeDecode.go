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
	// f, err := os.Create(time.Now().Format("2006-01-02T150405.pprof"))
	// if err != nil {
	// 	panic(err)
	// }
	// defer f.Close()

	// if err := trace.Start(f); err != nil {
	// 	panic(err)
	// }
	// defer trace.Stop()

	rand.Seed(time.Now().UnixNano())

	var errorProb float64 = 0
	linkDelay := 3000 * time.Millisecond

	// The links
	l1 := mpthSim.NewLink(errorProb, linkDelay)
	go l1.ProcessPackets()
	l2 := mpthSim.NewLink(errorProb, linkDelay)
	go l2.ProcessPackets()

	l3 := mpthSim.NewLink(errorProb, 2*linkDelay)
	go l3.ProcessPackets()
	l4 := mpthSim.NewLink(errorProb, 2*linkDelay)
	go l4.ProcessPackets()

	l5 := mpthSim.NewLink(errorProb, 4*linkDelay)
	go l5.ProcessPackets()
	l6 := mpthSim.NewLink(errorProb, 4*linkDelay)
	go l6.ProcessPackets()

	// The coders
	// Set the number of symbols (i.e. the generation size in RLNC
	// terminology) and the size of a symbol in bytes
	var symbols, symbolSize uint32 = 40, 1000
	var rate uint64 = 5000

	// Initialization of encoder and decoder
	encoderFactory := kodo.NewEncoderFactory(kodo.FullVector,
		kodo.Binary8, symbols, symbolSize)
	decoderFactory := kodo.NewDecoderFactory(kodo.FullVector,
		kodo.Binary8, symbols, symbolSize)

	// These lines show the API to clean the memory used by the factories
	defer kodo.DeleteEncoderFactory(encoderFactory)
	defer kodo.DeleteDecoderFactory(decoderFactory)

	encoderNode := mpthSim.NewEncoderNode(encoderFactory, rate)
	encoderNode.AddOutput(l1)
	// encoderNode.AddOutput(l3)
	// encoderNode.AddOutput(l5)

	// Just for fun - fill the data with random data
	for i := range encoderNode.Data {
		encoderNode.Data[i] = uint8(rand.Uint32())
	}
	encoderNode.SetConstSymbols()

	recoder1 := mpthSim.NewRecoderNode(decoderFactory, rate)
	recoder1.AddInput(l1)
	recoder1.AddOutput(l2)

	// recoder2 := mpthSim.NewRecoderNode(decoderFactory, rate)
	// recoder2.AddInput(l3)
	// recoder2.AddOutput(l4)
	// recoder3 := mpthSim.NewRecoderNode(decoderFactory, rate)
	// recoder3.AddInput(l5)
	// recoder3.AddOutput(l6)

	decoderNode := mpthSim.NewDecoderNode(decoderFactory, rate)
	decoderNode.AddInput(l2)
	// decoderNode.AddInput(l4)
	// decoderNode.AddInput(l6)

	var wg sync.WaitGroup
	wg.Add(1)
	// go decoderNode.ReceiveCodedPackets(&wg, encoderNode.Done, recoder1.Done, recoder2.Done, recoder3.Done)
	go decoderNode.ReceiveCodedPackets(&wg, encoderNode.Done, recoder1.Done)

	go recoder1.RecodeAndSend()
	// go recoder2.RecodeAndSend()
	// go recoder3.RecodeAndSend()

	go encoderNode.SendEncodedPackets()

	// Reset the recoder 1 after some time
	// go func() {
	// 	<-time.After(1000 * time.Millisecond)
	// 	fmt.Println("Reseting Recoder")
	// 	recoder1.Reset(decoderFactory)

	// 	l1 := mpthSim.NewLink(errorProb, linkDelay)
	// 	go l1.ProcessPackets()
	// 	l2 := mpthSim.NewLink(errorProb, linkDelay)
	// 	go l2.ProcessPackets()

	// 	<-time.After(time.Second)

	// 	recoder1.AddInput(l1)
	// 	encoderNode.AddOutput(l1)
	// 	decoderNode.AddInput(l2)
	// 	recoder1.AddOutput(l2)
	// 	go recoder1.RecodeAndSend()

	// }()

	wg.Wait()
	// Check if we properly decoded the data
	for i, v := range encoderNode.Data {
		if v != decoderNode.Data[i] {
			fmt.Println("Unexpected failure to decode")
			fmt.Println("Please file a bug report :)")
			return
		}
	}
	fmt.Println("Data decoded correctly")
	fmt.Println("Encoder Transmissions: ", encoderNode.Transmissions)
	fmt.Println("Recoder1 Transmissions: ", recoder1.Transmissions)
	// fmt.Println("Recoder2 Transmissions: ", recoder2.Transmissions)
	// fmt.Println("Recoder3 Transmissions: ", recoder3.Transmissions)
}

// func Wrapper(f func(), name string, wg *sync.WaitGroup) {
// 	f()
// 	fmt.Println(name, "returned")
// 	wg.Done()
// }
