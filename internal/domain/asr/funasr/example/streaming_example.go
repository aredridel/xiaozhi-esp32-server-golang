package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"

	"xiaozhi-esp32-server-golang/internal/domain/asr/funasr"
)

// readWavFile read WAV file and convert to PCM []float32 data
func readWavFile(filePath string) ([]float32, error) {
	// open WAV file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open WAV file failed: %v", err)
	}
	defer file.Close()

	// create WAV decoder
	wavDecoder := wav.NewDecoder(file)
	if !wavDecoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}

	// read WAV file info
	wavDecoder.ReadInfo()
	format := wavDecoder.Format()

	fmt.Printf("WAV format: sampling rate=%dHz, channel count=%d\n",
		int(format.SampleRate), format.NumChannels)

	// read all PCM data
	var allPcmData []float32

	// use 20ms frame size as buffer
	perFrameDuration := 20
	frameSize := int(format.SampleRate) * perFrameDuration / 1000
	audioBuf := &audio.IntBuffer{
		Format:         format,
		SourceBitDepth: 16,
		Data:           make([]int, frameSize*format.NumChannels),
	}

	fmt.Printf("use frame size: %d sampling point (%.1fms)\n", frameSize, float64(perFrameDuration))
	fmt.Println("start read WAV data...")

	for {
		// read WAV data
		n, err := wavDecoder.PCMBuffer(audioBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read WAV data failed: %v", err)
		}

		// convert int data to float32 (range -1.0 to 1.0)
		for i := 0; i < n; i++ {
			// convert int to float32, range from [-32768, 32767] to [-1.0, 1.0]
			floatSample := float32(audioBuf.Data[i]) / 32767.0
			allPcmData = append(allPcmData, floatSample)
		}
	}

	fmt.Printf("successful read WAV file, total sampling point count: %d, duration: %.2f second\n",
		len(allPcmData), float64(len(allPcmData))/float64(format.SampleRate))

	return allPcmData, nil
}

func main() {
	// define command line parameters
	var (
		host = flag.String("host", "192.168.208.214", "FunASR server IP address")
		port = flag.String("port", "10096", "FunASR server port")
		mode = flag.String("mode", "offline", "recognition pattern (online/offline)")
		file = flag.String("file", "test.wav", "WAV file path to recognize")
	)

	// parse command line parameters
	flag.Parse()

	// show usage instructions
	if len(os.Args) < 2 {
		fmt.Println("usage: ./streaming_example [option]")
		fmt.Println("options:")
		flag.PrintDefaults()
		fmt.Println("\nexamples:")
		fmt.Println("  ./streaming_example -host=192.168.1.100 -port=10095 -file=audio.wav")
		fmt.Println("  ./streaming_example -mode=online -file=test.wav")
		return
	}

	config := funasr.FunasrConfig{
		Host:          *host,
		Port:          *port,
		Mode:          *mode,
		SampleRate:    16000,
		ChunkSize:     []int{5, 10, 5},
		ChunkInterval: 10,
		Timeout:       30,
		AutoEnd:       false,
	}

	// use config create ASR instance
	asr, err := funasr.NewFunasr(config)
	if err != nil {
		fmt.Printf("create ASR instance failed: %v\n", err)
		return
	}

	fmt.Printf("target server: %s:%s, pattern: %s\n", config.Host, config.Port, config.Mode)

	// use command line parameter specified audio file path
	audioFilePath := *file

	// inspect audio file whether exists
	if _, err := os.Stat(audioFilePath); os.IsNotExist(err) {
		fmt.Printf("audio file %s does not exist\n", audioFilePath)
		fmt.Println("please provide valid audio file path")
		return
	}

	// read WAV file and convert to PCM data
	pcmData, err := readWavFile(audioFilePath)
	if err != nil {
		fmt.Printf("read WAV file failed: %v\n", err)
		return
	}

	// execute recognition
	result, err := asr.Process(pcmData)
	if err != nil {
		fmt.Printf("recognize failed: %v\n", err)
		return
	}

	// format and print result
	fmt.Println("recognition result:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println(result)
	fmt.Println(strings.Repeat("-", 40))
}
