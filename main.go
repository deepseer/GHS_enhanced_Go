package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

// Graph object in the algorithm
var Graph graphType
var logger *log.Logger

// GHS algorithm for Distributed Minimal Spanning Tree

func main() {
	var InputFileName = flag.String("i", "input.txt", "Input file name")
	var LogFileName = flag.String("o", "output.txt", "Output file name")
	var StartNode = flag.Int("s", 0, "Start from node X (-1 = all nodes wake up simutaneously)")
	var Latency = flag.Int("l", 5, "Simulated network latency (random number from 1 to N in milliseconds")
	flag.Parse()

	if *Latency < 1 || *Latency > 1000 {
		fmt.Println("Latency must be in [1, 1000]")
		*Latency = 5
	}

	var err error

	LogFile, err := os.Create(*LogFileName)
	if err != nil {
		fmt.Println("Initialization log file error")
		return
	}
	logger = log.New(LogFile, "", log.Lmicroseconds)

	err = Graph.InitializeGraph(*InputFileName)
	if err != nil {
		fmt.Println("Initialization graph error")
		return
	}
	fmt.Printf("Graph initialized, size = %d\n", Graph.Size)
	fmt.Printf("Simulated network latency: 1~%d (ms)\n", *Latency)

	if *StartNode < -1 || *StartNode >= Graph.Size {
		fmt.Println("Start node X must be bewteen -1 (all simuataneously) and Graph.Size-1. Default value: -1")
		*StartNode = -1
	}

	Option := optionType{
		StartNode: *StartNode,
		Latency:   *Latency,
	}
	err = Graph.StartAlgorithm(Option)
	if err != nil {
		fmt.Println("Start algorithm failed")
	}
}
