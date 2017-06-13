package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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

	// The factories
	encoderFactory := kodo.NewEncoderFactory(kodo.FullVector,
		kodo.Binary8, uint32(symbols), uint32(symbolSize))
	decoderFactory := kodo.NewDecoderFactory(kodo.FullVector,
		kodo.Binary8, uint32(symbols), uint32(symbolSize))
	// These lines show the API to clean the memory used by the factories
	defer kodo.DeleteEncoderFactory(encoderFactory)
	defer kodo.DeleteDecoderFactory(decoderFactory)

	res := &Result{}

	for i := uint(0); i < runs; i++ {

		// The links
		links := make([]*mpthSim.Link, 6)
		for i := 0; i < 6; i++ {
			links[i] = mpthSim.NewLink(losses[i], delays[i])
			go links[i].ProcessPackets()
		}

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
			recoders[i].NodeID = byte(i)
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

		start := time.Now()
		go encoderNode.SendEncodedPackets()

		mres := make([]float64, 3)
		mdown := make([]float64, 3)
		// Reset the recoders after their time expires
		reseter := func(i int) {
			if resets[i] == 0 {
				return
			}
			tRes := time.Now()
			<-time.After(resets[i])
			tDown := time.Now()
			mres[i] = time.Since(tRes).Seconds()
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

			decoderNode.AddInput(links[idx+1])
			encoderNode.AddOutput(links[idx])
			go recoders[i].RecodeAndSend()
			mdown[i] = time.Since(tDown).Seconds()

		}
		for i := range recoders {
			go reseter(i)
		}

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

		// Store results
		runTime := time.Since(start).Seconds()
		res.Latency = append(res.Latency, runTime)
		res.RxPackets = append(res.RxPackets, decoderNode.RxPackets)
		res.Symbols = append(res.Symbols, symbols)
		res.SymbolSize = append(res.SymbolSize, symbolSize)
		var ures []float64
		for _, x := range resets {
			ures = append(ures, x.Seconds())
		}
		res.UserResets = append(res.UserResets, ures)
		res.MeasuredResets = append(res.MeasuredResets, mres)
		var udown []float64
		for _, x := range downtimes {
			udown = append(udown, x.Seconds())
		}
		res.UserDowntimes = append(res.UserDowntimes, udown)
		res.MeasuredDowntimes = append(res.MeasuredDowntimes, mdown)
		res.Run = append(res.Run, i)
	}

	myres, err := json.Marshal(res)
	if err != nil {
		log.Fatal(err)
	}
	// fmt.Println(string(myres))
	// End Store results
	err = ioutil.WriteFile(time.Now().Format("2006-01-02_15:04")+"_simm.json", myres, 0644)

}

type Result struct {
	Run               []uint
	Symbols           []uint
	SymbolSize        []uint
	rate              []uint64
	UserResets        [][]float64 `json:"UserResets[s]"`
	MeasuredResets    [][]float64 `json:"MeasuredResets[s]"`
	UserDowntimes     [][]float64 `json:"UserDowntimes[s]"`
	MeasuredDowntimes [][]float64 `json:"MeasuredDowntimes[s]"`
	Latency           []float64   `json:"Latency[s]"`
	RxPackets         [][]uint32  `json:"RxPackets"`
}
