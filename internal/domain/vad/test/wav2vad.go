package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/streamer45/silero-vad-go/speech"
	"gopkg.in/hraban/opus.v2"
)

// readCloserWrapper is bytes.Reader provide Close method to implement ReadCloser interface
type readCloserWrapper struct {
	*bytes.Reader
}

// Close implement io.Closer interface
func (r *readCloserWrapper) Close() error {
	return nil
}

// newReadCloserWrapper create a new ReadCloser wrapper
func newReadCloserWrapper(data []byte) *readCloserWrapper {
	return &readCloserWrapper{bytes.NewReader(data)}
}

// WavToOpus convert WAV audio data to standard Opus format
// return Opus frame slice, each slice is an Opus encoded frame
func WavToOpus(wavData []byte, sampleRate int, channels int, bitRate int) ([][]byte, error) {

	sd, err := speech.NewDetector(speech.DetectorConfig{
		ModelPath:            "silero_vad.onnx",
		SampleRate:           16000,
		Threshold:            0.5,
		MinSilenceDurationMs: 250,
		SpeechPadMs:          150,
	})
	if err != nil {
		log.Fatalf("failed to create speech detector: %s", err)
	}

	// create WAV decoder
	wavReader := bytes.NewReader(wavData)
	wavDecoder := wav.NewDecoder(wavReader)
	if !wavDecoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}

	// read WAV file info
	wavDecoder.ReadInfo()
	format := wavDecoder.Format()
	wavSampleRate := int(format.SampleRate)
	wavChannels := int(format.NumChannels)

	// if provided parameter and file parameter not consistent, use file parameter
	if sampleRate == 0 {
		sampleRate = wavSampleRate
	}
	if channels == 0 {
		channels = wavChannels
	}

	// print wavDecoder info
	fmt.Println("WAV format:", format)

	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("create Opus encoder failed: %v", err)
	}

	dec, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("create Opus decoder failed: %v", err)
	}

	// set bitrate
	if bitRate > 0 {
		if err := enc.SetBitrate(bitRate); err != nil {
			return nil, fmt.Errorf("set bitrate failed: %v", err)
		}
	}

	// create output frame slice array
	opusFrames := make([][]byte, 0)

	perFrameDuration := 60
	// PCM buffer - Opus frame size (60ms)
	frameSize := sampleRate * perFrameDuration / 1000
	pcmBuffer := make([]int16, frameSize*channels)
	pcmBufferFloat32 := make([]float32, frameSize*channels)
	opusBuffer := make([]byte, 1000) // large enough buffer to store encoded data

	// read audio buffer
	audioBuf := &audio.IntBuffer{Data: make([]int, frameSize*channels), Format: format}

	fmt.Println("start convert...")

	pcmAllData := make([]float32, 0)
	for {
		// read WAV data
		n, err := wavDecoder.PCMBuffer(audioBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read WAV data failed: %v", err)
		}

		// convert int to int16
		for i := 0; i < len(audioBuf.Data); i++ {
			if i < len(pcmBuffer) {
				pcmBuffer[i] = int16(audioBuf.Data[i])
			}
		}

		// encode to Opus format
		n, err = enc.Encode(pcmBuffer, opusBuffer)
		if err != nil {
			return nil, fmt.Errorf("encode failed: %v", err)
		}

		// copy current frame to new slice and add to frame array
		frameData := make([]byte, n)
		copy(frameData, opusBuffer[:n])
		opusFrames = append(opusFrames, frameData)

		// decode opus to pcm
		n, err = dec.DecodeFloat32(frameData, pcmBufferFloat32)
		if err != nil {
			return nil, fmt.Errorf("decode failed: %v", err)
		}

		fmt.Printf("pcmBufferFloat32 len: %d\n", len(pcmBufferFloat32[:n]))

		segments, err := sd.Detect(pcmBufferFloat32[:n])
		if err != nil {
			//log.Fatalf("Detect failed: %s", err)
		}
		fmt.Printf("detect voice: %v\n", segments)

		pcmAllData = append(pcmAllData, pcmBufferFloat32[:n]...)
	}

	segments, err := sd.Detect(pcmAllData)
	if err != nil {
		log.Fatalf("Detect failed: %s", err)
	}
	fmt.Printf("detect voice: %v\n", segments)

	// output frameData to output.opus
	opusFile, err := os.OpenFile("output.opus", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to create opus file: %s", err)
	}
	opusFile.Write(opusFrames[0])
	opusFile.Close()

	/*
		// output pcm data to test.pcm
		pcmFile, err := os.OpenFile("test.pcm", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("failed to create pcm file: %s", err)
		}

		defer pcmFile.Close()
		dec, err := opus.NewDecoder(sampleRate, channels)
		if err != nil {
			return nil, fmt.Errorf("create Opus decoder failed: %v", err)
		}

		pcmBuffer = make([]int16, 10240)
		for _, data := range opusFrames {
			// decode opus data to pcm
			n, err := dec.Decode(data, pcmBuffer)
			if err != nil {
				return nil, fmt.Errorf("decode failed: %v", err)
			}
			frameData := make([]int16, len(pcmBuffer)*2)
			copy(frameData, pcmBuffer[:n])
			_, err = pcmFile.Write(frameData)
			if err != nil {
				log.Fatalf("failed to write to pcm file: %s", err)
			}
		}*/

	return opusFrames, nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("invalid arguments provided: expecting one file path")
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("failed to open sample audio file: %s", err)
	}
	defer f.Close()

	// read file all content
	mp3Data, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("failed to read mp3 file: %s", err)
	}

	// convert mp3 to opus
	opusData, err := WavToOpus(mp3Data, 16000, 1, 0)
	if err != nil {
		log.Fatalf("failed to convert mp3 to opus: %s", err)
	}

	// print opus data
	fmt.Printf("opusData: %d\n", len(opusData))

	// decode Opus data to pcm

	// output all data to test.opus
	/*opusFile, err := os.OpenFile("test.opus", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to create opus file: %s", err)
	}
	defer opusFile.Close()

	for _, data := range opusData {
		_, err := opusFile.Write(data)
		if err != nil {
			log.Fatalf("failed to write to opus file: %s", err)
		}
	}*/
}
