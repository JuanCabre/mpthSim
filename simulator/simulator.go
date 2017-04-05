package main

import (
	"math/rand"
	"time"

	"github.com/JuanCabre/mpthSim"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	l1 := mpthSim.NewLink(0.3, 50*time.Millisecond)
	go l1.ProcessPackets()

	encoderNode := mpthSim.Node{}
	encoderNode.AddOutput(l1.In)
	encoderNode.Done = make(chan struct{})

	decoderNode := mpthSim.Node{}
	decoderNode.AddInput(l1.Out)

	go decoderNode.ConsumePacket(10, encoderNode.Done)
	go encoderNode.GeneratePackets()

	for {
	}

}
