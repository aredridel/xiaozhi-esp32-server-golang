package manager

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"

	"xiaozhi-esp32-server-golang/internal/domain/asr"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	"xiaozhi-esp32-server-golang/internal/domain/tts"
	"xiaozhi-esp32-server-golang/internal/domain/vad/inter"
	"xiaozhi-esp32-server-golang/internal/pool"
	log "xiaozhi-esp32-server-golang/logger"
)

// DefaultTestWavPath configtestusefixed WAV path（16kHz 单声道，about 1–3 second），optional
const DefaultTestWavPath = "internal/testdata/config_test.wav"

// DefaultTestTextLLM/TTS fixedtesttext
const DefaultTestText = "configtest"
const (
	defaultLLMTestTimeout  = 15 * time.Second
	thinkingLLMTestTimeout = 30 * time.Second
)

// used for VAD/ASR ofbackup PCM：about 1 secondmockvoice 16kHz 单声道，nofilewhenuse
// use合成噪声mockrealvoice，以便 ASR server-side能normalprocess（特别yes Manual patternneed commit）
var fallbackPCM = func() []float32 {
	pcm := make([]float32, 16000)
	// generatemockvoicesignal：usemultiplepositive弦波叠加 + 噪声
	// mockin文"configtest"of基本frequencyrange
	// 增加幅degree以使server-side能recognizeisvalidaudio（Manual pattern要求更high）
	for i := range pcm {
		t := float64(i) / 16000.0
		// 基频 + 谐波mockvoice，large幅增加幅degree
		sample := float32(0.5 * math.Sin(2*math.Pi*t*400))   // 基频 400Hz，幅degree 0.5
		sample += float32(0.25 * math.Sin(2*math.Pi*t*800))  // 谐波，幅degree 0.25
		sample += float32(0.15 * math.Sin(2*math.Pi*t*1200)) // 谐波，幅degree 0.15
		sample += float32(0.1 * math.Sin(2*math.Pi*t*2000))  // 谐波，幅degree 0.1
		// add噪声，large幅增加噪声水平
		sample += (float32(i%100) - 50) / 2000 // 噪声幅degree增加to 0.05
		// applicationpackage络（淡入淡out）
		env := float32(1.0)
		if i < 1000 {
			env = float32(i) / 1000
		} else if i > 15000 {
			env = float32(16000-i) / 1000
		}
		pcm[i] = sample * env
	}
	log.Debugf("[config_test] fallbackPCM generate: len=%d", len(pcm))
	return pcm
}()

// loadTestWav loadfixed WAV is float32 PCM，iffileno存atthenreturn nil and nil error（call方use fallbackPCM）
func loadTestWav(path string) ([]float32, error) {
	if path == "" {
		path = DefaultTestWavPath
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	dec := wav.NewDecoder(f)
	if !dec.IsValidFile() {
		return nil, nil
	}
	dec.ReadInfo()
	wavFmt := dec.Format()
	frameSize := int(wavFmt.SampleRate) * 20 / 1000 * wavFmt.NumChannels
	buf := &audio.IntBuffer{Format: wavFmt, SourceBitDepth: 16, Data: make([]int, frameSize)}
	var out []float32
	for {
		n, err := dec.PCMBuffer(buf)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return nil, err
		}
		for i := 0; i < n; i++ {
			out = append(out, float32(buf.Data[i])/32767.0)
		}
	}
	return out, nil
}

// RunConfigTest according todown发of data（and实whenconfigconsistent）execute VAD/ASR/LLM/TTS 轻amounttest，return每class按 config_id ofresult
func RunConfigTest(data map[string]interface{}, testText string) (vadResult, asrResult, llmResult, ttsResult map[string]interface{}) {
	vadResult = make(map[string]interface{})
	asrResult = make(map[string]interface{})
	llmResult = make(map[string]interface{})
	ttsResult = make(map[string]interface{})

	if testText == "" {
		testText = DefaultTestText
	}
	log.Debugf("[config_test] RunConfigTest start test_text=%q data.keys=%v", testText, mapKeys(data))
	// 打印receivetoof各type config_id and脱敏afterofconfiginside容，便于 debug
	for _, typ := range []string{"vad", "asr", "llm", "tts"} {
		v, _ := data[typ].(map[string]interface{})
		if v == nil {
			continue
		}
		ids := make([]string, 0, len(v))
		for k := range v {
			ids = append(ids, k)
		}
		log.Debugf("[config_test] receive data[%s] config_ids=%v", typ, ids)
	}
	if redacted := redactSensitive(data); redacted != nil {
		if b, err := json.Marshal(redacted); err == nil {
			log.Debugf("[config_test] receive data 脱敏after: %s", string(b))
		}
	}

	pcm, _ := loadTestWav(DefaultTestWavPath)
	if pcm == nil || len(pcm) == 0 {
		log.Debugf("[config_test] WAV fileloadfailedorisempty，use fallbackPCM")
		pcm = fallbackPCM
	}
	log.Debugf("[config_test] use PCM data: len=%d", len(pcm))

	// VAD：countprocesstime consumption（fromcall IsVAD toreturn）
	if v, ok := data["vad"].(map[string]interface{}); ok {
		for configID, val := range v {
			if configID == "provider" {
				continue
			}
			cfg, ok := val.(map[string]interface{})
			if !ok {
				vadResult[configID] = map[string]interface{}{"ok": false, "message": "configformatinvalid"}
				continue
			}
			wrapper, err := pool.Acquire[inter.VAD]("vad", configID, cfg)
			if err != nil {
				vadResult[configID] = map[string]interface{}{"ok": false, "message": err.Error()}
				continue
			}
			vad := wrapper.GetProvider()
			t0 := time.Now()
			_, err = vad.IsVAD(pcm[:min(320, len(pcm))])
			elapsedMs := time.Since(t0).Milliseconds()
			pool.Release(wrapper)
			if err != nil {
				vadResult[configID] = map[string]interface{}{"ok": false, "message": err.Error(), "first_packet_ms": elapsedMs}
			} else {
				vadResult[configID] = map[string]interface{}{"ok": true, "message": "through", "first_packet_ms": elapsedMs}
			}
		}
	}

	// ASR：use StreamingRecognize do轻amounttest，countbodybodytime consumption
	if v, ok := data["asr"].(map[string]interface{}); ok {
		for configID, val := range v {
			if configID == "provider" {
				continue
			}
			cfg, ok := val.(map[string]interface{})
			if !ok {
				asrResult[configID] = map[string]interface{}{"ok": false, "message": "configformatinvalid"}
				log.Debugf("[config_test] ASR config_id=%s configformatinvalid", configID)
				continue
			}
			// resourcepool creator need引擎type（funasr/doubao），use config_id will报「unsupportedofASR引擎type」
			asrEngineType := "funasr"
			if p, ok := cfg["provider"].(string); ok && p != "" {
				asrEngineType = p
			}
			wrapper, err := pool.Acquire[asr.AsrProvider]("asr", asrEngineType, cfg)
			if err != nil {
				asrResult[configID] = map[string]interface{}{"ok": false, "message": err.Error()}
				continue
			}
			asrProvider := wrapper.GetProvider()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			audioCh := make(chan []float32)
			go func() {
				const chunk = 3200 // about 200ms @ 16kHz
				for i := 0; i < len(pcm); i += chunk {
					end := i + chunk
					if end > len(pcm) {
						end = len(pcm)
					}
					audioCh <- pcm[i:end]
				}
				close(audioCh)
			}()
			t0 := time.Now()
			resultChan, err := asrProvider.StreamingRecognize(ctx, audioCh)
			pool.Release(wrapper)
			if err != nil {
				cancel()
				asrResult[configID] = map[string]interface{}{"ok": false, "message": err.Error(), "first_packet_ms": time.Since(t0).Milliseconds()}
				continue
			}
			var asrErr error
			for r := range resultChan {
				if r.Error != nil {
					asrErr = r.Error
					break
				}
			}
			elapsedMs := time.Since(t0).Milliseconds()
			cancel()
			if asrErr != nil {
				asrResult[configID] = map[string]interface{}{"ok": false, "message": asrErr.Error(), "first_packet_ms": elapsedMs}
			} else {
				asrResult[configID] = map[string]interface{}{"ok": true, "message": "through", "first_packet_ms": elapsedMs}
			}
		}
	}

	//LLM
	if v, ok := data["llm"].(map[string]interface{}); ok {
		n := 0
		for k := range v {
			if k != "provider" {
				n++
			}
		}
		log.Debugf("[config_test]LLM 待测 config count: %d", n)
		for configID, val := range v {
			if configID == "provider" {
				continue
			}
			cfg, ok := val.(map[string]interface{})
			if !ok {
				llmResult[configID] = map[string]interface{}{"ok": false, "message": "configformatinvalid"}
				log.Debugf("[config_test]LLM config_id=%s configformatinvalid", configID)
				continue
			}
			testCfg := cloneConfigMap(cfg)
			testCfg["__enable_reasoning_content_detection"] = true
			wrapper, err := pool.Acquire[llm.LLMProvider]("llm", configID, testCfg)
			if err != nil {
				llmResult[configID] = map[string]interface{}{"ok": false, "message": err.Error()}
				log.Debugf("[config_test]LLM config_id=%s Acquire failed: %v", configID, err)
				continue
			}
			llmProvider := wrapper.GetProvider()
			timeout := defaultLLMTestTimeout
			if llmThinkingEnabled(cfg) {
				timeout = thinkingLLMTestTimeout
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			t0 := time.Now()
			msgChan := llmProvider.ResponseWithContext(ctx, "config_test", []*schema.Message{
				{Role: "user", Content: testText},
			}, nil)
			var gotMessage bool
			var firstMsg *schema.Message
			var firstPacketMs int64
			for msg := range msgChan {
				if msg != nil {
					firstMsg = msg
					gotMessage = true
					firstPacketMs = time.Since(t0).Milliseconds()
					break
				}
			}
			cancel()
			pool.Release(wrapper)
			resultBase := map[string]interface{}{"first_packet_ms": firstPacketMs}
			if reporter, ok := llmProvider.(interface{ HasReasoningContent() bool }); ok {
				resultBase["reasoning_content_returned"] = reporter.HasReasoningContent()
			}
			if gotMessage && llm.IsLLMErrorMessage(firstMsg) {
				errMsg := llm.LLMErrorMessage(firstMsg)
				resultBase["ok"] = false
				resultBase["message"] = errMsg
				llmResult[configID] = resultBase
				log.Debugf("[config_test]LLM config_id=%s failed(透传error): %s", configID, errMsg)
			} else if gotMessage {
				resultBase["ok"] = true
				resultBase["message"] = "through"
				llmResult[configID] = resultBase
				log.Debugf("[config_test]LLM config_id=%s through", configID)
			} else if ctx.Err() == context.DeadlineExceeded {
				resultBase["ok"] = false
				resultBase["message"] = "timeout"
				llmResult[configID] = resultBase
				log.Debugf("[config_test]LLM config_id=%s timeout", configID)
			} else {
				resultBase["ok"] = false
				resultBase["message"] = "not receivedtorespondorcallfailed"
				llmResult[configID] = resultBase
				log.Debugf("[config_test]LLM config_id=%s failed(not receivedtorespond)", configID)
			}
		}
	} else {
		log.Debugf("[config_test]LLM data.llm 缺失ornon map, ok=%v", ok)
	}

	// TTS
	if v, ok := data["tts"].(map[string]interface{}); ok {
		for configID, val := range v {
			if configID == "provider" {
				continue
			}
			cfg, ok := val.(map[string]interface{})
			if !ok {
				ttsResult[configID] = map[string]interface{}{"ok": false, "message": "configformatinvalid"}
				continue
			}
			wrapper, err := pool.Acquire[tts.TTSProvider]("tts", configID, cfg)
			if err != nil {
				ttsResult[configID] = map[string]interface{}{"ok": false, "message": err.Error()}
				continue
			}
			ttsProvider := wrapper.GetProvider()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			outputChan, err := ttsProvider.TextToSpeechStream(ctx, testText, 24000, 1, 60)
			if err != nil {
				cancel()
				pool.Release(wrapper)
				ttsResult[configID] = map[string]interface{}{"ok": false, "message": err.Error()}
				log.Warnf("TTS config test %s: %v", configID, err)
				continue
			}
			t0 := time.Now()
			var totalBytes int
			var firstPacketMs int64 = -1
			for chunk := range outputChan {
				if chunk != nil {
					if firstPacketMs < 0 {
						firstPacketMs = time.Since(t0).Milliseconds()
					}
					totalBytes += len(chunk)
				}
			}
			cancel()
			pool.Release(wrapper)
			if firstPacketMs < 0 {
				firstPacketMs = time.Since(t0).Milliseconds()
			}
			if totalBytes == 0 {
				ttsResult[configID] = map[string]interface{}{"ok": false, "message": "not receivedtovalidaudioor合成failed", "first_packet_ms": firstPacketMs}
				log.Debugf("[config_test] TTS config_id=%s failed(not receivedtovalidaudio)", configID)
			} else {
				ttsResult[configID] = map[string]interface{}{"ok": true, "message": "through", "first_packet_ms": firstPacketMs}
			}
		}
	}

	return vadResult, asrResult, llmResult, ttsResult
}

func cloneConfigMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func llmThinkingEnabled(cfg map[string]interface{}) bool {
	rawThinking, ok := cfg["thinking"].(map[string]interface{})
	if !ok || len(rawThinking) == 0 {
		return false
	}

	mode, _ := rawThinking["mode"].(string)
	mode = strings.ToLower(strings.TrimSpace(mode))
	return mode != "" && mode != "default"
}

// mapKeys return map ofkeylist，used for debug log
func mapKeys(m map[string]interface{}) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// 敏感fieldname（smallwrite），脱敏afterused forlog
var sensitiveKeys = map[string]bool{
	"api_key": true, "access_token": true, "token": true, "password": true, "secret": true,
}

// redactSensitive 深拷贝 data andwill敏感fieldvaluereplaceis "***"，used for debug log
func redactSensitive(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}
	out := make(map[string]interface{}, len(data))
	for k, v := range data {
		out[k] = redactValue(v)
	}
	return out
}

func redactValue(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(x))
		for k, val := range x {
			if sensitiveKeys[strings.ToLower(k)] {
				m[k] = "***"
			} else {
				m[k] = redactValue(val)
			}
		}
		return m
	case []interface{}:
		arr := make([]interface{}, len(x))
		for i, val := range x {
			arr[i] = redactValue(val)
		}
		return arr
	default:
		return v
	}
}
