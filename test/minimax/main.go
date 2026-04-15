package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"xiaozhi-esp32-server-golang/internal/domain/tts/minimax"
)

func main() {
	// Parse command line arguments
	text := flag.String("text", "真正的危险不是计算机开始像人一样思考，而是人开始像计算机一样思考。计算机只是可以帮我们处理一些简单事务。", "Text to synthesize")
	outputFile := flag.String("output", "output.mp3", "Output audio file name")
	apiKey := flag.String("api_key", "", "Minimax API Key (if not provided, will be read from MINIMAX_API_KEY environment variable)")
	model := flag.String("model", "speech-2.8-hd", "Model name")
	voiceID := flag.String("voice", "male-qn-qingse", "Voice ID")
	format := flag.String("format", "mp3", "Audio format (mp3/wav/pcm)")
	flag.Parse()

	// Get API Key
	if *apiKey == "" {
		*apiKey = os.Getenv("MINIMAX_API_KEY")
		if *apiKey == "" {
			fmt.Fprintf(os.Stderr, "Error: Please provide API Key (via -api_key parameter or MINIMAX_API_KEY environment variable)\n")
			os.Exit(1)
		}
	}

	// Create configuration
	config := map[string]interface{}{
		"api_key":     *apiKey,
		"model":       *model,
		"voice_id":    *voiceID,
		"speed":       1.0,
		"vol":         1.0,
		"pitch":       0,
		"sample_rate": 32000,
		"bitrate":     128000,
		"format":      *format,
		"channel":     1,
	}

	// Create Minimax TTS Provider
	provider := minimax.NewMinimaxTTSProvider(config)

	// Create context with cancellation support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling, support Ctrl+C interrupt
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		fmt.Println("\nReceived interrupt signal, canceling...")
		cancel()
	}()

	fmt.Printf("Starting text synthesis: %s\n", *text)
	fmt.Printf("Using configuration: model=%s, voice=%s, format=%s\n", *model, *voiceID, *format)
	fmt.Println("Connecting and synthesizing audio...")

	// Call streaming TTS
	startTime := time.Now()
	outputChan, err := provider.TextToSpeechStream(ctx, *text, 16000, 1, 60)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TTS synthesis failed: %v\n", err)
		os.Exit(1)
	}

	// Collect audio data
	var audioFrames [][]byte
	chunkCount := 0

	fmt.Println("Starting to receive audio data...")
	for frame := range outputChan {
		chunkCount++
		audioFrames = append(audioFrames, frame)
		if chunkCount%10 == 0 {
			fmt.Printf("Received %d audio frames...\n", chunkCount)
		}
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Audio synthesis complete! Received %d audio frames in %v\n", chunkCount, elapsed)

	// Merge all audio frames
	if len(audioFrames) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No audio data received\n")
		os.Exit(1)
	}

	// Calculate total size
	totalSize := 0
	for _, frame := range audioFrames {
		totalSize += len(frame)
	}

	// Merge audio data
	audioData := make([]byte, 0, totalSize)
	for _, frame := range audioFrames {
		audioData = append(audioData, frame...)
	}

	fmt.Printf("Total audio data size: %d bytes (%.2f KB)\n", totalSize, float64(totalSize)/1024)

	// Save to file
	// Note: The system internally uses Opus encoding, so received audio frames are in Opus format
	// If other formats (like WAV/MP3) are needed, additional decoding and encoding steps are required
	// Here we directly save Opus frames, which can be played with Opus-supporting players (like VLC, ffplay, etc.)

	fmt.Printf("Saving audio to file: %s\n", *outputFile)
	fmt.Println("Note: Saving Opus-encoded audio frames, can be played with Opus-supporting players")
	if err := os.WriteFile(*outputFile, audioData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Audio successfully saved to: %s\n", *outputFile)
	fmt.Printf("File size: %d bytes (%.2f KB)\n", len(audioData), float64(len(audioData))/1024)

	// Clean up resources
	if err := provider.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close Provider: %v\n", err)
	}

	fmt.Println("Test complete!")
}
