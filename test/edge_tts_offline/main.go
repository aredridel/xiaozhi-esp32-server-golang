package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/gorilla/websocket"
)

// Create WAV file header
func createWAVHeader(dataSize uint32) []byte {
	// WAV header total 44 bytes
	header := make([]byte, 44)

	// RIFF chunk descriptor
	copy(header[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:8], 36+dataSize) // Total file length minus 8 bytes
	copy(header[8:12], []byte("WAVE"))

	// fmt sub-chunk
	copy(header[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:20], 16)      // fmt chunk size
	binary.LittleEndian.PutUint16(header[20:22], 1)       // Audio format (1 = PCM)
	binary.LittleEndian.PutUint16(header[22:24], 1)       // Number of channels (1 = mono)
	binary.LittleEndian.PutUint32(header[24:28], 24000)   // Sample rate (24kHz)
	binary.LittleEndian.PutUint32(header[28:32], 24000*2) // Byte rate (SampleRate * BlockAlign)
	binary.LittleEndian.PutUint16(header[32:34], 2)       // Block align (channels * bits per sample / 8)
	binary.LittleEndian.PutUint16(header[34:36], 16)      // Bits per sample (16 bits)

	// data sub-chunk
	copy(header[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:44], dataSize) // Audio data size

	return header
}

func main() {
	// Parse command line parameters
	url := flag.String("url", "ws://192.168.208.214:8081", "WebSocket server address")
	flag.Parse()

	// Connect to WebSocket server
	c, _, err := websocket.DefaultDialer.Dial(*url, nil)
	if err != nil {
		log.Fatal("Connection failed:", err)
	}
	defer c.Close()

	// Create channel for handling interrupt signals
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Create channel for detecting interrupt
	done := make(chan struct{})

	// Listen for interrupt signals in background
	go func() {
		<-interrupt
		fmt.Println("\nReceived interrupt signal, closing connection...")

		// Gracefully close connection
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("Error occurred while closing connection:", err)
		}
		close(done)
	}()

	// Create standard input reader
	reader := bufio.NewReader(os.Stdin)
	fileCount := 1

	fmt.Println("Please enter text to convert (type 'exit' to quit):")
	for {
		select {
		case <-done:
			return
		default:
			// Read user input
			fmt.Print("> ")
			text, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Failed to read input:", err)
				continue
			}

			// Trim whitespace from input text
			text = strings.TrimSpace(text)

			// Check if exit
			if text == "exit" {
				fmt.Println("Program exiting...")
				return
			}

			// If input is empty, continue to next iteration
			if text == "" {
				continue
			}

			// Send message
			err = c.WriteMessage(websocket.TextMessage, []byte(text))
			if err != nil {
				log.Println("Failed to send message:", err)
				continue
			}
			fmt.Printf("Message sent: %s\n", text)

			// Receive message
			msgType, data, err := c.ReadMessage()
			if err != nil {
				log.Println("Failed to receive message:", err)
				continue
			}
			fmt.Printf("Received data length: %d bytes, type: %d\n", len(data), msgType)

			// Generate unique filename
			filename := fmt.Sprintf("voice_%d.wav", fileCount)
			fileCount++

			// Create WAV header
			wavHeader := createWAVHeader(uint32(len(data)))

			// Create complete WAV file data
			fullData := make([]byte, len(wavHeader)+len(data))
			copy(fullData[0:], wavHeader)
			copy(fullData[len(wavHeader):], data)

			// Save complete WAV file
			err = os.WriteFile(filename, fullData, 0644)
			if err != nil {
				log.Println("Failed to save file:", err)
				continue
			}
			fmt.Printf("Audio data saved to %s\n", filename)
		}
	}
}
