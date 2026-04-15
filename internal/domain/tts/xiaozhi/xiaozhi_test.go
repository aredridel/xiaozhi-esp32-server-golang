package xiaozhi

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/hraban/opus.v2"

	"xiaozhi-esp32-server-golang/internal/util/workqueue"
)

func OpusToWav(opusData [][]byte, sampleRate int, channels int, fileName string) ([][]int16, error) {
	opusDecoder, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("create Opus decoder failed: %v", err)
	}

	wavOut, err := os.Create(fileName)
	if err != nil {
		return nil, fmt.Errorf("create WAV file failed: %v", err)
	}

	pcmDataList := make([][]int16, 0)
	pcmBuffer := make([]int16, 4096)

	wavEncoder := wav.NewEncoder(wavOut, sampleRate, 16, channels, 1)
	wavBuffer := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: channels, // use passed channel count
			SampleRate:  sampleRate,
		},
		SourceBitDepth: 16,
		Data:           make([]int, 4096),
	}

	for _, frame := range opusData {
		n, err := opusDecoder.Decode(frame, pcmBuffer)
		if err != nil {
			return nil, fmt.Errorf("decode failed: %v", err)
		}
		copyData := make([]int16, len(pcmBuffer[:n]))
		copy(copyData, pcmBuffer[:n])
		pcmDataList = append(pcmDataList, copyData)

		//fmt.Println("pcmData len: ", len(copyData))

		// convert PCM data to int format
		for i := 0; i < len(copyData); i++ {
			wavBuffer.Data = append(wavBuffer.Data, int(copyData[i]))
		}
	}

	// write WAV file
	err = wavEncoder.Write(&wavBuffer)
	if err != nil {
		return nil, fmt.Errorf("write WAV file failed: %v", err)
	}

	wavEncoder.Close()

	return pcmDataList, nil
}

func initLog() error {
	// use standard output instead of file
	logrus.SetOutput(os.Stdout)

	// disable default caller report, use custom caller field
	logrus.SetReportCaller(false)
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000", // time format, add millisecond
		ForceColors:     true,                      // enable color output
	})
	logLevel, _ := logrus.ParseLevel(viper.GetString("log.level"))
	if logLevel == 0 {
		logLevel = logrus.DebugLevel // default set to Debug level
	}
	logrus.SetLevel(logLevel)
	return nil
}

func TestTextToSpeechStream(t *testing.T) {
	// initialize log output to standard output
	//initLog()
	provider := NewXiaozhiProvider(map[string]interface{}{
		"server_addr": "wss://api.tenclass.net/xiaozhi/v1/",
		"device_id":   "ba:8f:17:de:94:94",
	})

	textList := []string{
		"Hello, Xiaozhi TTS unit test",
		"Tell a joke",
		"How is the weather today",
		"What is your name",
		"How old are you this year",
		"Where do you live",
		"What do you like to eat",
		"What is your favorite color",
		"What is your favorite food",
		"What is your favorite animal",
	}

	workqueue.ParallelizeUntil(context.Background(), 3, len(textList), func(piece int) {
		text := textList[piece]
		fmt.Println("start speech text: ", text)
		ch, err := provider.TextToSpeechStream(context.Background(), text)
		if err != nil {
			fmt.Println("TextToSpeechStream failed: ", err)
			return
		}
		opusDataList := [][]byte{}
		for frame := range ch {
			opusDataList = append(opusDataList, frame)
			if len(frame) == 0 {
				t.Error("received empty audio frame")
			}
		}
		fmt.Printf("text: %s, received %d audio frames\n", text, len(opusDataList))
	})

	/*
		for _, text := range textList {
			fmt.Println("start speech text: ", text)
			ch, err := provider.TextToSpeechStream(context.Background(), text)
			if err != nil {
				fmt.Println("TextToSpeechStream joinfailed: ", err)
				return
			}
			opusDataList := [][]byte{}
			for frame := range ch {
				opusDataList = append(opusDataList, frame)
				if len(frame) == 0 {
					t.Error("receiveemptyaudio frame")
				}
			}
			//OpusToWav(opusDataList, 24000, 1, "output_24000.wav")
		}*/

}
