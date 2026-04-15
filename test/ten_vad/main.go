package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"xiaozhi-esp32-server-golang/internal/domain/audio"
	"xiaozhi-esp32-server-golang/internal/domain/vad/ten_vad"

	goaudio "github.com/go-audio/audio"
	"github.com/go-audio/wav"
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
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <wav_file_path> [hop_size] [threshold]\nExample: %s test.wav 512 0.3", os.Args[0], os.Args[0])
	}

	wavFilePath := os.Args[1]

	// Parse optional parameters
	hopSize := 512
	threshold := 0.3
	if len(os.Args) >= 3 {
		_, err := fmt.Sscanf(os.Args[2], "%d", &hopSize)
		if err != nil {
			log.Printf("Invalid hop_size parameter, using default value 512")
			hopSize = 512
		}
	}
	if len(os.Args) >= 4 {
		_, err := fmt.Sscanf(os.Args[3], "%f", &threshold)
		if err != nil {
			log.Printf("Invalid threshold parameter, using default value 0.3")
			threshold = 0.3
		}
	}

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
	// Use standard parameters supported by TEN-VAD: 16000Hz sample rate, mono
	sampleRate := 16000
	channels := 1

	pcmFloat32, pcmBytes, err := Wav2Pcm(wavData, sampleRate, channels)
	if err != nil {
		log.Fatalf("WAV to PCM conversion failed: %v", err)
	}

	_ = pcmBytes

	fmt.Printf("Successfully converted to PCM data, total %d frames (20ms per frame)\n", len(pcmFloat32))

	// Create TEN-VAD instance
	config := map[string]interface{}{
		"hop_size":  hopSize,
		"threshold": threshold,
	}
	vadImpl, err := ten_vad.NewTenVAD(config)
	if err != nil {
		log.Fatalf("Failed to create TEN-VAD: %v", err)
	}
	defer vadImpl.Close()

	fmt.Printf("TEN-VAD created successfully (hop_size=%d, threshold=%.2f), starting test...\n", hopSize, threshold)

	// Directly test if VAD can work properly
	if len(pcmFloat32) == 0 {
		log.Fatalf("No PCM data available for processing")
	}

	// Merge all frames into continuous audio data
	// Because TEN-VAD needs to frame by hopSize, not by 20ms
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

	// Frame by hopSize for detection
	detectVoice := func(pcmData []float32) {
		speechFrames := 0
		totalFrames := 0
		var speechFramesData []float32 // Collect all speech frames

		// Frame by hopSize processing
		for i := 0; i < len(pcmData); i += hopSize {
			end := i + hopSize
			if end > len(pcmData) {
				end = len(pcmData)
			}

			frame := pcmData[i:end]

			// If frame length is less than hopSize, pad with zeros or skip
			if len(frame) < hopSize {
				// Pad with zeros to hopSize length
				paddedFrame := make([]float32, hopSize)
				copy(paddedFrame, frame)
				frame = paddedFrame
			}

			totalFrames++

			// Perform VAD detection
			isVoice, err := vadImpl.IsVADExt(frame, sampleRate, hopSize)
			if err != nil {
				log.Printf("Frame %d VAD detection failed: %v", totalFrames, err)
				// If first frame fails, VAD is not properly initialized
				if totalFrames == 1 {
					log.Fatalf("VAD initialization failed, please check TEN-VAD configuration and library files")
				}
				continue
			}

			if isVoice {
				speechFrames++
				// Collect speech frame data (use original frame, without padding)
				originalFrame := pcmData[i:end]
				speechFramesData = append(speechFramesData, originalFrame...)
				fmt.Printf("Frame %d: Voice activity detected (sample range: %d-%d)\n", totalFrames, i, end-1)
			} else {
				fmt.Printf("Frame %d: No voice activity (sample range: %d-%d)\n", totalFrames, i, end-1)
			}
		}

		// Output statistics
		speechPercentage := float64(speechFrames) / float64(totalFrames) * 100
		nonSpeechFrames := totalFrames - speechFrames
		fmt.Printf("\n=== TEN-VAD Detection Results Statistics ===\n")
		fmt.Printf("Total frames: %d (per frame %d samples, %.2f ms)\n", totalFrames, hopSize, float64(hopSize)/float64(sampleRate)*1000)
		fmt.Printf("Speech frames: %d\n", speechFrames)
		fmt.Printf("Non-speech frames: %d\n", nonSpeechFrames)
		fmt.Printf("Voice activity ratio: %.2f%%\n", speechPercentage)

		if speechFrames > 0 {
			fmt.Println("Conclusion: Voice activity detected")

			// Save speech frames to WAV file
			outputFileName := generateOutputFileName(wavFilePath)
			err := saveFloat32ToWav(speechFramesData, outputFileName, sampleRate, channels)
			if err != nil {
				log.Printf("Failed to save speech frames to WAV file: %v", err)
			} else {
				fmt.Printf("Successfully saved speech frames to: %s (total %d samples, %.2f seconds)\n",
					outputFileName, len(speechFramesData), float64(len(speechFramesData))/float64(sampleRate))
			}
		} else {
			fmt.Println("Conclusion: No voice activity detected")
		}
	}

	// Test using merged complete audio data
	detectVoice(allPcmData)
}

func float32ToByte(pcmFrame []float32) []byte {
	byteData := make([]byte, len(pcmFrame)*4)
	for i, sample := range pcmFrame {
		binary.LittleEndian.PutUint32(byteData[i*4:], math.Float32bits(sample))
	}
	return byteData
}

// generateOutputFileName generates output file name, adding "_speech" suffix to original file name
func generateOutputFileName(inputPath string) string {
	dir := filepath.Dir(inputPath)
	baseName := filepath.Base(inputPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	outputName := nameWithoutExt + "_speech" + ext
	return filepath.Join(dir, outputName)
}

// saveFloat32ToWav saves float32 PCM data as WAV file
func saveFloat32ToWav(pcmData []float32, fileName string, sampleRate int, channels int) error {
	// Create output file
	wavFile, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create WAV file: %v", err)
	}
	defer wavFile.Close()

	// Create WAV encoder
	wavEncoder := wav.NewEncoder(wavFile, sampleRate, 16, channels, 1)

	// Convert float32 to int16
	// float32 range is typically [-1.0, 1.0], need to scale to int16 range [-32768, 32767]
	intData := make([]int, len(pcmData))
	for i, sample := range pcmData {
		// Limit range to [-1.0, 1.0]
		if sample > 1.0 {
			sample = 1.0
		}
		if sample < -1.0 {
			sample = -1.0
		}
		// Convert to int16 range
		intSample := int(sample * 32767.0)
		if intSample > 32767 {
			intSample = 32767
		}
		if intSample < -32768 {
			intSample = -32768
		}
		intData[i] = intSample
	}

	// Create audio buffer
	audioBuf := &goaudio.IntBuffer{
		Format: &goaudio.Format{
			NumChannels: channels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: 16,
		Data:           intData,
	}

	// Write to WAV file
	err = wavEncoder.Write(audioBuf)
	if err != nil {
		return fmt.Errorf("failed to write WAV file: %v", err)
	}

	// Close encoder
	err = wavEncoder.Close()
	if err != nil {
		return fmt.Errorf("failed to close WAV encoder: %v", err)
	}

	return nil
}
