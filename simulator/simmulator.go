package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"gitlab.com/steinwurf/kodo-go/src/kodo"

	"github.com/JuanCabre/mpthSim"
)

func main() {

	rand.Seed(time.Now().UnixNano()) // Seed the RNG

	flag.Parse()
	verifyFlags() // Verify if the flags were correctly set

	// The links
	links := make([]*mpthSim.Link, 6)
	for i := 0; i < 6; i++ {
		links[i] = mpthSim.NewLink(losses[i], delays[i])
		go links[i].ProcessPackets()
	}

	// The factories
	encoderFactory := kodo.NewEncoderFactory(kodo.FullVector,
		kodo.Binary8, uint32(symbols), uint32(symbolSize))
	decoderFactory := kodo.NewDecoderFactory(kodo.FullVector,
		kodo.Binary8, uint32(symbols), uint32(symbolSize))
	// These lines show the API to clean the memory used by the factories
	defer kodo.DeleteEncoderFactory(encoderFactory)
	defer kodo.DeleteDecoderFactory(decoderFactory)

	// Create the encoder node...
	encoderNode := mpthSim.NewEncoderNode(encoderFactory, rate)
	// ...add the outputs...
	for i := range links {
		if i%2 == 0 {
			encoderNode.AddOutput(links[i])
		}
	}
	// ...and fill the encoder with random data
	for i := range encoderNode.Data {
		encoderNode.Data[i] = uint8(rand.Uint32())
	}
	encoderNode.SetConstSymbols()

	// Create the recoder nodes
	var recoders []*mpthSim.Node
	linkCount := 0
	for i := 0; i < 3; i++ {
		recoders = append(recoders, mpthSim.NewRecoderNode(decoderFactory, rate))
		recoders[i].AddInput(links[linkCount])
		recoders[i].AddOutput(links[linkCount+1])
		linkCount += 2
	}

	// Create the decoder node
	decoderNode := mpthSim.NewDecoderNode(decoderFactory, rate)
	for i := range links {
		if i%2 == 1 {
			decoderNode.AddInput(links[i])
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go decoderNode.ReceiveCodedPackets(&wg, encoderNode.Done,
		recoders[0].Done, recoders[1].Done, recoders[2].Done)
	// go decoderNode.ReceiveCodedPackets(&wg, encoderNode.Done, recoder1.Done)

	for _, r := range recoders {
		go r.RecodeAndSend()
	}

	res := &Result{}
	start := time.Now()
	go encoderNode.SendEncodedPackets()

	// Reset the recoders after their time expires
	reseter := func(i int) {
		if resets[i] == 0 {
			return
		}
		<-time.After(resets[i])
		fmt.Println("Reseting Recoder")
		recoders[i].Reset(decoderFactory)

		// Input link
		idx := 2 * i
		links[idx] = mpthSim.NewLink(losses[idx], delays[idx])
		go links[idx].ProcessPackets()
		recoders[i].AddInput(links[idx])

		// Output link
		links[idx+1] = mpthSim.NewLink(losses[idx+1], delays[idx+1])
		go links[idx+1].ProcessPackets()
		recoders[i].AddOutput(links[idx+1])

		<-time.After(downtimes[i])

		go recoders[i].RecodeAndSend()
		decoderNode.AddInput(links[idx+1])
		encoderNode.AddOutput(links[idx])

	}
	for i := range recoders {
		reseter(i)
	}

	wg.Wait()

	runTime := time.Since(start).Seconds()
	res.Latency = append(res.Latency, runTime)
	res.RxPackets = append(res.RxPackets, 100)

	myres, err := json.Marshal(res)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(myres))

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
	for i := range recoders {
		fmt.Printf("Recoder %d sent %d packets\n", i, recoders[i].Transmissions)
	}
}

type Result struct {
	Latency   []float64 `json:"Latency[s]"`
	RxPackets []uint32  `json:"RxPackets"`
}

// func NewResult() *Result {
// 	r := new(Result)
// 	r.Latency = make([]float64, 0)
// }
