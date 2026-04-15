package main

import (
	"fmt"
	"os"

	"github.com/hraban/opus"
)

func main() {
	// Audio parameter settings
	channels := 1
	sampleRate := 16000 // 16kHz
	fmt.Printf("Channels: %d, Sample Rate: %d Hz\n", channels, sampleRate)

	// Create an encoder with VoIP application type (low latency voice)
	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppVoIP)
	if err != nil {
		fmt.Printf("Failed to create encoder: %v\n", err)
		os.Exit(1)
	}

	// Set bitrate to 16kbps
	if err = enc.SetBitrate(16000); err != nil {
		fmt.Printf("Failed to set bitrate: %v\n", err)
		os.Exit(1)
	}

	// Set complexity, 0-10 range, higher is better quality but more CPU usage
	if err = enc.SetComplexity(5); err != nil {
		fmt.Printf("Failed to set complexity: %v\n", err)
		os.Exit(1)
	}

	// Generate 20ms test PCM data (20ms per frame, 16kHz sample rate = 320 samples)
	frameSize := 320
	pcm := make([]int16, frameSize*channels)

	// Generate a simple sine wave for testing
	for i := 0; i < frameSize; i++ {
		// Simple sine wave, frequency around 440Hz
		value := int16(10000.0 * float64(i%36) / 36.0)
		pcm[i] = value
	}

	// Store encoded data
	data := make([]byte, 1000)

	// Encode PCM data to Opus
	n, err := enc.Encode(pcm, data)
	if err != nil {
		fmt.Printf("Encoding failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Encoded %d samples to %d bytes of Opus data, compression ratio: %.2f%%\n",
		frameSize*channels, n, float64(n)/float64(frameSize*channels*2)*100)

	// Create decoder for decoding test
	dec, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		fmt.Printf("Failed to create decoder: %v\n", err)
		os.Exit(1)
	}

	// Store decoded PCM data
	decodedPCM := make([]int16, frameSize*channels)

	// Decode Opus data to PCM
	samplesDecoded, err := dec.Decode(data[:n], decodedPCM)
	if err != nil {
		fmt.Printf("Decoding failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Decoded %d bytes of Opus data to %d samples\n", n, samplesDecoded)

	// Calculate difference between original and decoded PCM
	var sumDiff int64
	for i := 0; i < frameSize; i++ {
		diff := int64(pcm[i]) - int64(decodedPCM[i])
		if diff < 0 {
			diff = -diff
		}
		sumDiff += diff
	}
	avgDiff := float64(sumDiff) / float64(frameSize)

	fmt.Printf("Average difference between original and decoded PCM: %.2f\n", avgDiff)
	fmt.Println("Opus codec example completed!")
}
