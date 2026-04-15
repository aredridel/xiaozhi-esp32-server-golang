package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Read file, use parameters for input and output files
	inputFilePath := flag.String("input", "", "Input file path")
	outputFilePath := flag.String("output", "", "Output file path")
	sampleRate := flag.Int("sampleRate", 24000, "Sample rate")
	channels := flag.Int("channels", 1, "Number of channels")
	flag.Parse()

	if *inputFilePath == "" || *outputFilePath == "" {
		flag.Usage()
		return
	}

	// Read all file content
	content, err := os.ReadFile(*inputFilePath)
	if err != nil {
		fmt.Println("Failed to read file:", err)
		return
	}

	fmt.Println("Successfully read file:", *inputFilePath)

	opusData := [][]byte{content}
	pcmData, err := OpusToWav(opusData, *sampleRate, *channels, *outputFilePath)
	if err != nil {
		fmt.Println("Conversion failed:", err)
		return
	}
	fmt.Println("pcmData len: ", len(pcmData[0]))

	fmt.Println("Conversion successful:", *outputFilePath)
}
