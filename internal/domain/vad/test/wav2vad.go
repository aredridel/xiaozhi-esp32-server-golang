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

// readCloserWrapper is bytes.Reader provide Close method以implement ReadCloser interface
type readCloserWrapper struct {
	*bytes.Reader
}

// Close implement io.Closer interface
func (r *readCloserWrapper) Close() error {
	return nil
}

// newReadCloserWrapper create anew ReadCloser package装
func newReadCloserWrapper(data []byte) *readCloserWrapper {
	return &readCloserWrapper{bytes.NewReader(data)}
}

// WavToOpus willWAVaudio dataconvertisstandardOpusformat
// returnOpusframeofsliceset，每个sliceyesaOpusencodeframe
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

	// createWAVdecode器
	wavReader := bytes.NewReader(wavData)
	wavDecoder := wav.NewDecoder(wavReader)
	if !wavDecoder.IsValidFile() {
		return nil, fmt.Errorf("invalidofWAVfile")
	}

	// readWAVfileinfo
	wavDecoder.ReadInfo()
	format := wavDecoder.Format()
	wavSampleRate := int(format.SampleRate)
	wavChannels := int(format.NumChannels)

	// ifprovideofparameterandfileparameternoconsistent，usefileinofparameter
	if sampleRate == 0 {
		sampleRate = wavSampleRate
	}
	if channels == 0 {
		channels = wavChannels
	}

	//打印wavDecoderinfo
	fmt.Println("WAVformat:", format)

	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("createOpusencode器failed: %v", err)
	}

	dec, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("createOpusencode器failed: %v", err)
	}

	// set比特率
	if bitRate > 0 {
		if err := enc.SetBitrate(bitRate); err != nil {
			return nil, fmt.Errorf("set比特率failed: %v", err)
		}
	}

	// createoutputframeslicearray
	opusFrames := make([][]byte, 0)

	perFrameDuration := 60
	// PCMbuffer区 - Opusframesize(60ms)
	frameSize := sampleRate * perFrameDuration / 1000
	pcmBuffer := make([]int16, frameSize*channels)
	pcmBufferFloat32 := make([]float32, frameSize*channels)
	opusBuffer := make([]byte, 1000) // 足够largeofbuffer区storeencodeafterofdata

	// readaudiobuffer区
	audioBuf := &audio.IntBuffer{Data: make([]int, frameSize*channels), Format: format}

	fmt.Println("startconvert...")

	pcmAllData := make([]float32, 0)
	for {
		// readWAVdata
		n, err := wavDecoder.PCMBuffer(audioBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("readWAVdatafailed: %v", err)
		}

		// willintconvertisint16
		for i := 0; i < len(audioBuf.Data); i++ {
			if i < len(pcmBuffer) {
				pcmBuffer[i] = int16(audioBuf.Data[i])
			}
		}

		// encodeisOpusformat
		n, err = enc.Encode(pcmBuffer, opusBuffer)
		if err != nil {
			return nil, fmt.Errorf("encodefailed: %v", err)
		}

		// willcurrentframereplicationtonewsliceinandaddtoframearray
		frameData := make([]byte, n)
		copy(frameData, opusBuffer[:n])
		opusFrames = append(opusFrames, frameData)

		//willopusdecode至pcm
		n, err = dec.DecodeFloat32(frameData, pcmBufferFloat32)
		if err != nil {
			return nil, fmt.Errorf("decodefailed: %v", err)
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

	//willframeDataoutput至test.opus
	opusFile, err := os.OpenFile("output.opus", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to create opus file: %s", err)
	}
	opusFile.Write(opusFrames[0])
	opusFile.Close()

	/*
		//willpcmdataoutput至test.pcm
		pcmFile, err := os.OpenFile("test.pcm", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("failed to create pcm file: %s", err)
		}

		defer pcmFile.Close()
		dec, err := opus.NewDecoder(sampleRate, channels)
		if err != nil {
			return nil, fmt.Errorf("createOpusdecode器failed: %v", err)
		}

		pcmBuffer = make([]int16, 10240)
		for _, data := range opusFrames {
			//willopusdatadecode成pcm
			n, err := dec.Decode(data, pcmBuffer)
			if err != nil {
				return nil, fmt.Errorf("decodefailed: %v", err)
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

	//readfileallinside容
	mp3Data, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("failed to read mp3 file: %s", err)
	}

	//willmp3convertisopus
	opusData, err := WavToOpus(mp3Data, 16000, 1, 0)
	if err != nil {
		log.Fatalf("failed to convert mp3 to opus: %s", err)
	}

	//打印opusdata
	fmt.Printf("opusData: %d\n", len(opusData))

	//willOpusdatadecode成pcm

	//willalldataoutput至test.opus
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
