package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"

	"xiaozhi-esp32-server-golang/internal/domain/audio"
	"xiaozhi-esp32-server-golang/internal/domain/vad/webrtc_vad"
)

func genFloat32Empty(sampleRate int, durationMs int, channels int, count int) [][]float32 {
	// Calculate number of samples
	numSamples := int(float64(sampleRate) * float64(durationMs) / 1000.0)
	// Create silence buffer
	var buf bytes.Buffer
	// 32-bit float silence value is 0.0
	for i := 0; i < numSamples*channels; i++ {
		binary.Write(&buf, binary.LittleEndian, float32(0.0))
	}
	// Convert data to float32
	float32Data := make([]float32, numSamples*channels)
	for i := 0; i < numSamples*channels; i++ {
		float32Data[i] = float32(buf.Bytes()[i])
	}
	result := make([][]float32, 0)
	for i := 0; i < count; i++ {
		result = append(result, float32Data)
	}
	return result
}

func genOpusFloat32Empty(sampleRate int, durationMs int, channels int, count int) [][]float32 {
	// Calculate number of samples
	numSamples := int(float64(sampleRate) * float64(durationMs) / 1000.0)

	audioProcesser, err := audio.GetAudioProcesser(sampleRate, channels, 20)
	if err != nil {
		fmt.Printf("Failed to get decoder: %v", err)
		return nil
	}

	pcmFrame := make([]int16, numSamples)

	opusFrame := make([]byte, 1000)
	n, err := audioProcesser.Encoder(pcmFrame, opusFrame)
	if err != nil {
		fmt.Printf("Decoding failed: %v", err)
		return nil
	}

	// Convert opus data to float32
	pcmFloat32 := make([]float32, n)
	for i := 0; i < n; i++ {
		pcmFloat32[i] = float32(opusFrame[i])
	}

	result := make([][]float32, 0)
	for i := 0; i < count; i++ {
		tmp := make([]float32, n)
		copy(tmp, pcmFloat32)
		result = append(result, tmp)
	}
	return result
}

func main() {
	// Check command line arguments
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <wav_file_path>", os.Args[0])
	}

	wavFilePath := os.Args[1]

	// Read WAV file
	wavFile, err := os.Open(wavFilePath)
	if err != nil {
		log.Fatalf("Failed to open WAV file: %v", err)
	}
	defer wavFile.Close()

	// Read entire file content
	wavData, err := io.ReadAll(wavFile)
	if err != nil {
		log.Fatalf("Failed to read WAV file: %v", err)
	}

	fmt.Printf("Successfully read WAV file: %s (%d bytes)\n", wavFilePath, len(wavData))

	// Call Wav2Pcm function to convert WAV data to PCM data
	// Use standard parameters supported by WebRTC VAD: 16000Hz sample rate, mono
	sampleRate := 16000
	channels := 1

	pcmFloat32, pcmBytes, err := Wav2Pcm(wavData, sampleRate, channels)
	if err != nil {
		log.Fatalf("WAV to PCM conversion failed: %v", err)
	}

	_ = pcmBytes

	fmt.Printf("Successfully converted to PCM data, total %d frames (20ms per frame)\n", len(pcmFloat32))

	// Create WebRTC VAD instance
	vadImpl, err := webrtc_vad.NewWebRTCVADWithConfig(sampleRate, 2) // Mode 2: medium sensitivity
	if err != nil {
		log.Fatalf("Failed to create WebRTC VAD: %v", err)
	}
	defer vadImpl.Close()

	fmt.Println("WebRTC VAD created successfully, starting test...")

	// Directly test if VAD can work properly
	if len(pcmFloat32) == 0 {
		log.Fatalf("No PCM data available for processing")
	}

	// WebRTC VAD requires 320 samples (20ms @ 16000Hz) per frame
	// Wav2Pcm already frames by 20ms, each frame is exactly 320 samples
	frameSize := 320 // 20ms @ 16000Hz

	// Merge all frames into continuous audio data, then reframe by frameSize
	// This ensures each frame is a complete frameSize
	totalSamples := 0
	for _, frame := range pcmFloat32 {
		totalSamples += len(frame)
	}
	allPcmData := make([]float32, 0, totalSamples)
	for _, frame := range pcmFloat32 {
		allPcmData = append(allPcmData, frame...)
	}

	fmt.Printf("Merged audio data: %d samples (%.2f seconds)\n", len(allPcmData), float64(len(allPcmData))/float64(sampleRate))

	// Check audio data range (for debugging)
	if len(allPcmData) > 0 {
		minVal := allPcmData[0]
		maxVal := allPcmData[0]
		for _, v := range allPcmData {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
		fmt.Printf("Audio data range: [%.6f, %.6f]\n", minVal, maxVal)
		// If data is not within [-1.0, 1.0] range, may need normalization
		if maxVal > 1.0 || minVal < -1.0 {
			fmt.Printf("Warning: Audio data exceeds [-1.0, 1.0] range, may need normalization\n")
		}
	}

	fmt.Println("Starting voice activity detection...")

	// Frame by frameSize for detection
	detectVoice := func(pcmData []float32) {
		speechFrames := 0
		totalFrames := 0

		// Frame by frameSize processing
		for i := 0; i < len(pcmData); i += frameSize {
			end := i + frameSize
			if end > len(pcmData) {
				end = len(pcmData)
			}

			frame := pcmData[i:end]

			// If frame length is less than frameSize, pad with zeros
			if len(frame) < frameSize {
				// Pad with zeros to frameSize length
				paddedFrame := make([]float32, frameSize)
				copy(paddedFrame, frame)
				frame = paddedFrame
			}

			totalFrames++

			// Perform VAD detection
			isVoice, err := vadImpl.IsVADExt(frame, sampleRate, frameSize)
			if err != nil {
				log.Printf("Frame %d VAD detection failed: %v", totalFrames, err)
				// If first frame fails, VAD is not properly initialized
				if totalFrames == 1 {
					log.Fatalf("VAD initialization failed, please check WebRTC VAD configuration and library files")
				}
				continue
			}

			if isVoice {
				speechFrames++
				fmt.Printf("Frame %d: Voice activity detected (sample range: %d-%d)\n", totalFrames, i, end-1)
			} else {
				fmt.Printf("Frame %d: No voice activity (sample range: %d-%d)\n", totalFrames, i, end-1)
			}
		}

		// Output statistics
		speechPercentage := float64(speechFrames) / float64(totalFrames) * 100
		nonSpeechFrames := totalFrames - speechFrames
		fmt.Printf("\n=== WebRTC VAD Detection Results Statistics ===\n")
		fmt.Printf("Total frames: %d (per frame %d samples, %.2f ms)\n", totalFrames, frameSize, float64(frameSize)/float64(sampleRate)*1000)
		fmt.Printf("Speech frames: %d\n", speechFrames)
		fmt.Printf("Non-speech frames: %d\n", nonSpeechFrames)
		fmt.Printf("Voice activity ratio: %.2f%%\n", speechPercentage)

		if speechFrames > 0 {
			fmt.Println("Conclusion: Voice activity detected")
		} else {
			fmt.Println("Conclusion: No voice activity detected")
		}
	}

	// Test using actual WAV file data
	detectVoice(allPcmData)
}

func float32ToByte(pcmFrame []float32) []byte {
	byteData := make([]byte, len(pcmFrame)*4)
	for i, sample := range pcmFrame {
		binary.LittleEndian.PutUint32(byteData[i*4:], math.Float32bits(sample))
	}
	return byteData
}
