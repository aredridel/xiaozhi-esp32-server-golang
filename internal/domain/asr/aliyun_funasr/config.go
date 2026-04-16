package aliyun_funasr

import (
	"os"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultWsURL          = "wss://dashscope.aliyuncs.com/api-ws/v1/inference/"
	defaultModel          = "fun-asr-realtime"
	defaultFormat         = "pcm"
	defaultSampleRate     = 16000
	defaultTimeoutSeconds = 30
)

// Config Aliyun FunASR config
type Config struct {
	APIKey                     string
	WsURL                      string
	Model                      string
	Format                     string
	SampleRate                 int
	VocabularyID               string
	DisfluencyRemovalEnabled   bool
	SemanticPunctuationEnabled bool
	Timeout                    time.Duration
}

// DefaultConfig return default config
func DefaultConfig() Config {
	return Config{
		WsURL:      defaultWsURL,
		Model:      defaultModel,
		Format:     defaultFormat,
		SampleRate: defaultSampleRate,
		Timeout:    time.Duration(defaultTimeoutSeconds) * time.Second,
	}
}

// ConfigFromMap from config map merge generate config (support config file + internal control system)
func ConfigFromMap(cfg map[string]interface{}) Config {
	conf := DefaultConfig()

	// first merge config file default values
	applyViperDefaults(&conf)

	// compatible with old format: if passed { aliyun_funasr: { ... } }, then priority take internal map
	if nested, ok := cfg["aliyun_funasr"].(map[string]interface{}); ok {
		cfg = nested
	}

	applyMapOverrides(&conf, cfg)

	// api_key allow empty when fallback to environment variable
	if conf.APIKey == "" {
		conf.APIKey = os.Getenv("DASHSCOPE_API_KEY")
	}

	return conf
}

func applyViperDefaults(conf *Config) {
	const prefix = "asr.aliyun_funasr."
	if viper.IsSet(prefix + "api_key") {
		conf.APIKey = viper.GetString(prefix + "api_key")
	}
	if viper.IsSet(prefix + "ws_url") {
		conf.WsURL = viper.GetString(prefix + "ws_url")
	}
	if viper.IsSet(prefix + "model") {
		conf.Model = viper.GetString(prefix + "model")
	}
	if viper.IsSet(prefix + "format") {
		conf.Format = viper.GetString(prefix + "format")
	}
	if viper.IsSet(prefix + "sample_rate") {
		if sr := viper.GetInt(prefix + "sample_rate"); sr > 0 {
			conf.SampleRate = sr
		}
	}
	if viper.IsSet(prefix + "vocabulary_id") {
		conf.VocabularyID = viper.GetString(prefix + "vocabulary_id")
	}
	if viper.IsSet(prefix + "disfluency_removal_enabled") {
		conf.DisfluencyRemovalEnabled = viper.GetBool(prefix + "disfluency_removal_enabled")
	}
	if viper.IsSet(prefix + "semantic_punctuation_enabled") {
		conf.SemanticPunctuationEnabled = viper.GetBool(prefix + "semantic_punctuation_enabled")
	}
	if viper.IsSet(prefix + "timeout") {
		if t := viper.GetInt(prefix + "timeout"); t > 0 {
			conf.Timeout = time.Duration(t) * time.Second
		}
	}
}

func applyMapOverrides(conf *Config, cfg map[string]interface{}) {
	if v, ok := cfg["api_key"].(string); ok && v != "" {
		conf.APIKey = v
	}
	if v, ok := cfg["ws_url"].(string); ok && v != "" {
		conf.WsURL = v
	}
	if v, ok := cfg["model"].(string); ok && v != "" {
		conf.Model = v
	}
	if v, ok := cfg["format"].(string); ok && v != "" {
		conf.Format = v
	}
	if v, ok := cfg["sample_rate"].(int); ok && v > 0 {
		conf.SampleRate = v
	} else if v, ok := cfg["sample_rate"].(float64); ok && v > 0 {
		conf.SampleRate = int(v)
	}
	if v, ok := cfg["vocabulary_id"].(string); ok && v != "" {
		conf.VocabularyID = v
	}
	if v, ok := cfg["disfluency_removal_enabled"].(bool); ok {
		conf.DisfluencyRemovalEnabled = v
	}
	if v, ok := cfg["semantic_punctuation_enabled"].(bool); ok {
		conf.SemanticPunctuationEnabled = v
	}
	if v, ok := cfg["timeout"].(int); ok && v > 0 {
		conf.Timeout = time.Duration(v) * time.Second
	} else if v, ok := cfg["timeout"].(float64); ok && v > 0 {
		conf.Timeout = time.Duration(int(v)) * time.Second
	}
}
