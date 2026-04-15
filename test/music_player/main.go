package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"xiaozhi-esp32-server-golang/internal/domain/play_music"
	log "xiaozhi-esp32-server-golang/logger"
)

func main() {
	// Check command line arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <music URL>")
		fmt.Println("Example: go run main.go https://example.com/music.mp3")
		os.Exit(1)
	}

	musicURL := os.Args[1]
	fmt.Printf("Starting to play music: %s\n", musicURL)

	// Create context with cancellation support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create music player configuration
	config := play_music.DefaultMusicPlayerConfig()
	config.FrameDuration = 20 // 20ms frame duration

	// Create music player
	player := play_music.NewMusicPlayer(config.ToMap())

	// Display player information
	playerInfo := player.GetPlayerInfo()
	fmt.Printf("Player info: %+v\n", playerInfo)

	// Set up signal handling for graceful exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start streaming playback
	audioChan, err := player.PlayMusicStream(ctx, musicURL)
	if err != nil {
		log.Errorf("Failed to start music playback: %v", err)
		return
	}

	fmt.Println("Music playback started, streaming audio data...")
	fmt.Println("Press Ctrl+C to stop playback")

	// Statistics
	stats := &play_music.StreamingStats{
		StartTime: time.Now().UnixMilli(),
	}

	// Start statistics goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fmt.Printf("\n=== Playback Statistics ===\n")
				fmt.Printf("Frames generated: %d\n", stats.FramesGenerated)
				fmt.Printf("Bytes decoded: %d\n", stats.BytesDecoded)
				fmt.Printf("Running time: %d seconds\n", (time.Now().UnixMilli()-stats.StartTime)/1000)
				fmt.Printf("First frame time: %d ms\n", stats.FirstFrameTime-stats.StartTime)
				fmt.Printf("===============\n")
			}
		}
	}()

	// Process audio stream data
	go func() {
		frameCount := 0
		totalBytes := 0
		firstFrame := true

		for {
			select {
			case <-ctx.Done():
				fmt.Println("Stopping audio stream processing")
				return

			case audioFrame, ok := <-audioChan:
				if !ok {
					fmt.Println("Audio stream ended")
					cancel() // Cancel context, end program
					return
				}

				if firstFrame {
					firstFrame = false
					stats.FirstFrameTime = time.Now().UnixMilli()
					fmt.Printf("Received first audio frame, size: %d bytes\n", len(audioFrame))
				}

				frameCount++
				totalBytes += len(audioFrame)
				stats.FramesGenerated = int64(frameCount)
				stats.BytesDecoded = int64(totalBytes)

				// Here you can send audio frames to audio output device or process otherwise
				// For example: send to client via WebSocket, write to audio file, etc.

				// Show progress every 100 frames
				if frameCount%100 == 0 {
					fmt.Printf("Processed %d frames, total %d bytes\n", frameCount, totalBytes)
				}
			}
		}
	}()

	// Wait for signal or context cancellation
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v, stopping playback...\n", sig)
		cancel()
	case <-ctx.Done():
		fmt.Println("Playback complete")
	}

	// Stop player
	if err := player.Stop(); err != nil {
		log.Errorf("Failed to stop player: %v", err)
	}

	fmt.Println("Player stopped")
	fmt.Printf("Final statistics: Total frames=%d, Total bytes=%d\n", stats.FramesGenerated, stats.BytesDecoded)
}

// Example: Write audio frames to file (optional)
func saveAudioFramesToFile(frames <-chan []byte, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	frameCount := 0
	for frame := range frames {
		_, err := file.Write(frame)
		if err != nil {
			return err
		}
		frameCount++
	}

	fmt.Printf("Written %d audio frames to file: %s\n", frameCount, filename)
	return nil
}

// Example: Send audio stream via WebSocket (pseudo-code)
func streamAudioViaWebSocket(frames <-chan []byte, wsURL string) {
	fmt.Printf("Simulating sending audio stream via WebSocket to: %s\n", wsURL)

	for frame := range frames {
		// This is pseudo-code, actual implementation needs WebSocket connection
		fmt.Printf("Sending audio frame: %d bytes\n", len(frame))

		// Simulate send delay
		time.Sleep(20 * time.Millisecond) // Corresponds to 20ms frame duration
	}
}
