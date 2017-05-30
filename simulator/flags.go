package main

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Flags

// Set the number of symbols (i.e. the generation size in RLNC
// terminology) and the size of a symbol in bytes
var symbols uint
var symbolSize uint
var rate uint64

// Create user defined flags
type loss []float64           // Loss probabilities
type interval []time.Duration // Delays

func (l *loss) String() string {
	return fmt.Sprint(*l)
}

func (i *interval) String() string {
	return fmt.Sprint(*i)
}

func (l *loss) Set(value string) error {
	if len(*l) > 0 {
		return errors.New("loss flag already set")
	}
	for _, dt := range strings.Split(value, ",") {
		loss, err := strconv.ParseFloat(dt, 64)
		if err != nil {
			return err
		}
		*l = append(*l, loss)
	}
	return nil
}

func (i *interval) Set(value string) error {
	if len(*i) > 0 {
		return errors.New("interval flag already set")
	}
	for _, dt := range strings.Split(value, ",") {
		duration, err := time.ParseDuration(dt)
		if err != nil {
			return err
		}
		*i = append(*i, duration)
	}
	return nil
}

var losses loss
var delays interval
var resets interval
var downtimes interval

func init() {
	flag.Var(&losses, "losses", "comma-separated lists of the loss probabilities of the links")
	flag.Var(&delays, "delays", "comma-separated lists of the delays of the links, e.g., 50ms,10ms,...")
	flag.Var(&resets, "resets", "comma-separated lists of the times before resetting the recoders, e.g., 2s, 5s,...")
	flag.Var(&downtimes, "downtimes", "comma-separated lists of the downtimes of the recoders, e.g., 2s, 5s,...")

	flag.UintVar(&symbols, "symbols", 40, "The generation size")
	flag.UintVar(&symbolSize, "symbolSize", 1000, "The symbol size")
	flag.Uint64Var(&rate, "rate", 5000, "the transmission rate")
}

// Verify if the flags given by the user had the right size. Otherwise, set the
// default values
func verifyFlags() {
	if len(losses) != 6 {
		fmt.Println("flag losses: Incorrect size. Setting it up to the default 0.0")
		losses = make([]float64, 6)
	}
	if len(delays) != 6 {
		fmt.Println("flag delays: Incorrect size. Setting it up to the default 0")
		delays = make([]time.Duration, 6)
	}
	if len(resets) != 3 {
		fmt.Println("flag resets: Incorrect size. Setting it up to the default 0")
		resets = make([]time.Duration, 3)
	}
	if len(downtimes) != 3 {
		fmt.Println("flag downtimes: Incorrect size. Setting it up to the default 0")
		downtimes = make([]time.Duration, 3)
	}
}
