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

// readWavFile readWAVfileandconvertisPCM []float32data
func readWavFile(filePath string) ([]float32, error) {
	// openWAVfile
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("openWAVfilefailed: %v", err)
	}
	defer file.Close()

	// createWAVdecode器
	wavDecoder := wav.NewDecoder(file)
	if !wavDecoder.IsValidFile() {
		return nil, fmt.Errorf("invalidofWAVfile")
	}

	// readWAVfileinfo
	wavDecoder.ReadInfo()
	format := wavDecoder.Format()

	fmt.Printf("WAVformat: sampling率=%dHz, channelcount=%d\n",
		int(format.SampleRate), format.NumChannels)

	// readallPCMdata
	var allPcmData []float32

	// use20msframesizeasisbuffer区
	perFrameDuration := 20
	frameSize := int(format.SampleRate) * perFrameDuration / 1000
	audioBuf := &audio.IntBuffer{
		Format:         format,
		SourceBitDepth: 16,
		Data:           make([]int, frameSize*format.NumChannels),
	}

	fmt.Printf("useframesize: %d samplingpoint (%.1fms)\n", frameSize, float64(perFrameDuration))
	fmt.Println("startreadWAVdata...")

	for {
		// readWAVdata
		n, err := wavDecoder.PCMBuffer(audioBuf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("readWAVdatafailed: %v", err)
		}

		// willintdataconvertisfloat32 (range-1.0to1.0)
		for i := 0; i < n; i++ {
			// willintconvertisfloat32，rangefrom[-32768, 32767]to[-1.0, 1.0]
			floatSample := float32(audioBuf.Data[i]) / 32767.0
			allPcmData = append(allPcmData, floatSample)
		}
	}

	fmt.Printf("successfulreadWAVfile，总samplingpointcount: %d, duration: %.2fsecond\n",
		len(allPcmData), float64(len(allPcmData))/float64(format.SampleRate))

	return allPcmData, nil
}

func main() {
	// 定义command行parameter
	var (
		host = flag.String("host", "192.168.208.214", "FunASRserverIPaddress")
		port = flag.String("port", "10096", "FunASRserverport")
		mode = flag.String("mode", "offline", "recognizepattern (online/offline)")
		file = flag.String("file", "test.wav", "要recognizeofWAVfilepath")
	)

	// parsecommand行parameter
	flag.Parse()

	// 显示useinstruction
	if len(os.Args) < 2 {
		fmt.Println("use法: ./streaming_example [option]")
		fmt.Println("option:")
		flag.PrintDefaults()
		fmt.Println("\nexample:")
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

	// useconfigcreateASRinstance
	asr, err := funasr.NewFunasr(config)
	if err != nil {
		fmt.Printf("createASRinstancefailed: %v\n", err)
		return
	}

	fmt.Printf("目标server: %s:%s, pattern: %s\n", config.Host, config.Port, config.Mode)

	// usecommand行parameterspecifyofaudiofilepath
	audioFilePath := *file

	// inspectaudiofilewhether存at
	if _, err := os.Stat(audioFilePath); os.IsNotExist(err) {
		fmt.Printf("audiofile %s no存at\n", audioFilePath)
		fmt.Println("pleaseprovidevalidofaudiofilepath")
		return
	}

	// readWAVfileandconvertisPCMdata
	pcmData, err := readWavFile(audioFilePath)
	if err != nil {
		fmt.Printf("readWAVfilefailed: %v\n", err)
		return
	}

	// executerecognize
	result, err := asr.Process(pcmData)
	if err != nil {
		fmt.Printf("recognize failed: %v\n", err)
		return
	}

	// formatand打印result
	fmt.Println("recognizeresult:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println(result)
	fmt.Println(strings.Repeat("-", 40))
}
