package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/wav"
	"gopkg.in/hraban/opus.v2"
)

func main1() {
	// HTTP API URL
	mp3URL := "http://home.hackers365.com:55555/apk/test.mp3"
	// Specify output PCM file path
	pcmFilePath := "output.pcm"

	// Create PCM file
	pcmFile, err := os.Create(pcmFilePath)
	if err != nil {
		fmt.Printf("Failed to create PCM file: %v\n", err)
		return
	}
	defer pcmFile.Close()

	// Get MP3 data from HTTP API and process
	err = processMP3FromHTTP(mp3URL, pcmFile)
	if err != nil {
		fmt.Printf("Failed to process HTTP MP3 data: %v\n", err)
		return
	}

	fmt.Printf("HTTP MP3 data successfully decoded to PCM format, saved to: %s\n", pcmFilePath)

	// Export to WAV format
	exportHTTPToWav(mp3URL, "output.wav")
}

type readCloserWrapper struct {
	io.Reader
}

func (r readCloserWrapper) Close() error {
	return nil
}

// Get and process MP3 data from HTTP API
func processMP3FromHTTP(url string, pcmFile *os.File) error {
	// Make HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request returned non-200 status code: %d", resp.StatusCode)
	}

	// Create a pipe for processing data stream
	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()

	// Create read buffer and sample buffer
	bufferSize := 10 * 1024            // 10KB
	buffer := make([]byte, bufferSize) // HTTP read buffer

	opusBuffer := make([]byte, 1000) // Opus encoding output buffer

	// Create error channel and done channel
	errChan := make(chan error, 1)
	doneChan := make(chan struct{}, 1)

	// Start goroutine to decode MP3 and process PCM
	go func() {
		// Try to initialize decoder
		streamer, format, err := mp3.Decode(pipeReader)
		if err != nil {
			errChan <- fmt.Errorf("MP3 decoder initialization failed: %v", err)
			return
		}
		defer streamer.Close()

		fmt.Printf("MP3 decoder initialized successfully, sample rate: %d Hz, channels: %d\n",
			format.SampleRate, format.NumChannels)

		// Original MP3 format info
		sampleRate := int(format.SampleRate)
		channels := int(format.NumChannels)

		// PCM buffer and Opus frame size (e.g., 60ms)
		perFrameDuration := 60 // milliseconds
		frameSize := sampleRate * perFrameDuration / 1000
		pcmBuffer := make([]int16, frameSize*channels)
		opusFrames := make([][]byte, 0) // Store encoded Opus frames

		enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
		if err != nil {
			fmt.Printf("Failed to create Opus encoder: %v\n", err)
			errChan <- fmt.Errorf("failed to create Opus encoder: %v", err)
			return
		}

		beepSampleBuf := make([][2]float64, 1024) // Beep decode buffer
		// Process decoded audio stream
		currentFramePos := 0 // Current position filled in pcmBuffer
		for {
			// Read samples from stream to sampleBuf
			numSamplesRead, ok := streamer.Stream(beepSampleBuf)
			if !ok {
				// Process remaining data less than one frame
				if currentFramePos > 0 {
					// Create a complete frame buffer, fill remaining with zeros
					paddedFrame := make([]int16, len(pcmBuffer))
					copy(paddedFrame, pcmBuffer[:currentFramePos]) // Copy valid data to beginning, remaining defaults to 0

					// Encode the padded complete frame
					n, err := enc.Encode(paddedFrame, opusBuffer)
					if err != nil {
						fmt.Printf("Failed to encode remaining data: %v\n", err)
						// May need to send error through errChan
					} else {
						frameData := make([]byte, n)
						copy(frameData, opusBuffer[:n])
						opusFrames = append(opusFrames, frameData)
						// Note: encoding a complete frame here, even if original data is insufficient
						fmt.Printf("Encoded final padded %d PCM samples (original %d)\n", len(paddedFrame), currentFramePos)
					}
				}
				// Decoding complete
				doneChan <- struct{}{}
				return
			}

			// Convert read float64 samples to int16 and fill pcmBuffer
			for i := 0; i < numSamplesRead; i++ {
				// Direct conversion
				leftSample := int16(beepSampleBuf[i][0] * 32767.0)
				rightSample := int16(beepSampleBuf[i][1] * 32767.0)

				// Write PCM data
				pcmBuffer[currentFramePos] = leftSample
				if channels > 1 {
					pcmBuffer[currentFramePos+1] = rightSample
				}
				currentFramePos += channels

				// If pcmBuffer is full for one frame, encode it
				if currentFramePos == len(pcmBuffer) {
					n, err := enc.Encode(pcmBuffer, opusBuffer)
					if err != nil {
						fmt.Printf("Encoding failed: %v\n", err)
						errChan <- fmt.Errorf("encoding failed: %v", err)
						return
					}

					// Copy current frame to new slice and add to frame array
					frameData := make([]byte, n)
					copy(frameData, opusBuffer[:n])
					opusFrames = append(opusFrames, frameData)

					fmt.Printf("Encoded one frame (%d PCM samples)\n", len(pcmBuffer))
					currentFramePos = 0 // Reset frame position
				}
			}
		}
	}()

	// Create timer, send data every 100ms
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Start loop to read HTTP data and write to pipe
	for {
		select {
		case <-ticker.C:
			// Read data from HTTP response
			n, err := resp.Body.Read(buffer)

			// If data is read, write to pipe
			if n > 0 {
				_, writeErr := pipeWriter.Write(buffer[:n])
				if writeErr != nil {
					return fmt.Errorf("failed to write to pipe: %v", writeErr)
				}
				fmt.Printf("Read and wrote %d bytes of MP3 data\n", n)
			}

			// Handle EOF or error
			if err != nil {
				if err == io.EOF {
					fmt.Println("HTTP data stream read complete")
					pipeWriter.Close() // Close pipe write end

					// Wait for decoding to complete or error
					select {
					case <-doneChan:
						return nil
					case err := <-errChan:
						return err
					}
				} else {
					return fmt.Errorf("error reading HTTP data: %v", err)
				}
			}

		case err := <-errChan:
			return err

		case <-doneChan:
			return nil
		}
	}
}

func exportHTTPToWav(url string, wavFilePath string) {
	// Make HTTP request
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("HTTP request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("HTTP request returned non-200 status code: %d\n", resp.StatusCode)
		return
	}

	// Decode MP3
	streamer, format, err := mp3.Decode(resp.Body)
	if err != nil {
		fmt.Printf("Failed to decode MP3 data: %v\n", err)
		return
	}
	defer streamer.Close()

	// Create WAV file
	wavFile, err := os.Create(wavFilePath)
	if err != nil {
		fmt.Printf("Failed to create WAV file: %v\n", err)
		return
	}
	defer wavFile.Close()

	// Use beep/wav package to encode stream to WAV
	err = wav.Encode(wavFile, streamer, format)
	if err != nil {
		fmt.Printf("WAV encoding failed: %v\n", err)
		return
	}

	fmt.Printf("Exported WAV file from HTTP: %s\n", wavFilePath)
}
