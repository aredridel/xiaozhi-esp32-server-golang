package controllers

import "strings"

// VoiceInfo describes a Qwen TTS voice
type VoiceInfo struct {
	Value       string   `json:"value"`       // API voice parameter, e.g. "Cherry"
	Label       string   `json:"label"`       // Display name, e.g. "Qianyue"
	Description string   `json:"description"` // Short description
	Languages   []string `json:"languages"`   // Supported languages
}

// ModelVoiceMap maps model family -> supported voice list
// Note: Grouped by model "family", e.g. qwen3-tts-flash* as one family, qwen-tts* as another.
var ModelVoiceMap = map[string][]VoiceInfo{
	// Tongyi Qwen3-TTS-Flash series (qwen3-tts-flash / qwen3-tts-flash-2025-11-27 / qwen3-tts-flash-2025-09-18)
	"qwen3-tts-flash": {
		{Value: "Cherry", Label: "Qianyue", Description: "Sunny, positive, and naturally friendly young lady (Female)"},
		{Value: "Serena", Label: "Suyao", Description: "Gentle young lady (Female)"},
		{Value: "Ethan", Label: "Chenxu", Description: "Standard Mandarin with some Northern accent, sunny, warm, and energetic (Male)"},
		{Value: "Chelsie", Label: "Qianxue", Description: "Anime virtual girlfriend (Female)"},
		{Value: "Momo", Label: "Motu", Description: "Playful and quirky, cheering you up (Female)"},
		{Value: "Vivian", Label: "Shisan", Description: "Cool and cute with a bit of temper (Female)"},
		{Value: "Moon", Label: "Yuebai", Description: "Straightforward and handsome Yuebai (Male)"},
		{Value: "Maia", Label: "Siyue", Description: "A blend of intellect and gentleness (Female)"},
		{Value: "Kai", Label: "Kai", Description: "An ear SPA experience (Male)"},
		{Value: "Nofish", Label: "No Fish", Description: "Designer who can't pronounce retroflex sounds (Male)"},
		{Value: "Bella", Label: "Mengbao", Description: "Little loli who doesn't do drunken boxing (Female)"},
		{Value: "Jennifer", Label: "Jennifer", Description: "Premium brand-quality American female voice with cinematic texture (Female)"},
		{Value: "Ryan", Label: "Tiancha", Description: "Full rhythm, explosive acting, dancing between reality and tension (Male)"},
		{Value: "Katerina", Label: "Katerina", Description: "Mature lady voice with memorable rhythm (Female)"},
		{Value: "Aiden", Label: "Aiden", Description: "American big boy who excels at cooking (Male)"},
		{Value: "Eldric Sage", Label: "Cangmingzi", Description: "Steady and wise elder, weathered like pine yet clear as mirror (Male)"},
		{Value: "Mia", Label: "Guaixiaomei", Description: "Gentle as spring water, obedient as first snow (Female)"},
		{Value: "Mochi", Label: "Shaxiaomi", Description: "Clever and bright little adult, childlike yet precocious as Zen (Male)"},
		{Value: "Bellona", Label: "Yanzhengying", Description: "Resonant voice, clear articulation, vivid character (Female)"},
		{Value: "Vincent", Label: "Tianshu", Description: "Hoarse smoky voice telling tales of armies and rivers and lakes (Male)"},
		{Value: "Bunny", Label: "Mengxiaoji", Description: "\"Moe\" attribute bursting little loli (Female)"},
		{Value: "Neil", Label: "Awen", Description: "Most professional news anchor (Male)"},
		{Value: "Elias", Label: "Mojiangshi", Description: "Rigorous yet narrative lecturer voice (Female)"},
		{Value: "Arthur", Label: "Xudaye", Description: "Simple voice soaked by years and dry tobacco (Male)"},
		{Value: "Nini", Label: "Linjiamimei", Description: "Voice as soft and sticky as mochi (Female)"},
		{Value: "Ebona", Label: "Guipopo", Description: "Slightly horror-style grandmother voice (Female)"},
		{Value: "Seren", Label: "Xiaowan", Description: "Gentle and soothing, sleep-aid voice (Female)"},
		{Value: "Pip", Label: "Wanpixiaohai", Description: "Naughty and mischievous yet full of childlike innocence (Male)"},
		{Value: "Stella", Label: "Shaonvayue", Description: "Usually sweet to the point of cloying, full of justice when it matters (Female)"},
		{Value: "Bodega", Label: "Bodega", Description: "Passionate Spanish uncle (Male)"},
		{Value: "Sonrisa", Label: "Sonrisa", Description: "Warm and cheerful Latina sister (Female)"},
		{Value: "Alek", Label: "Aolieke", Description: "Warm voice beneath the cold exterior of a fighting nation (Male)"},
		{Value: "Dolce", Label: "Duoerqie", Description: "Laid-back Italian uncle (Male)"},
		{Value: "Sohee", Label: "Suxi", Description: "Gentle, cheerful, and emotionally rich Korean sister (Female)"},
		{Value: "Ono Anna", Label: "Xiaoyexing", Description: "Witty and quirky childhood friend (Female)"},
		{Value: "Lenn", Label: "Laien", Description: "German youth with rationality as base and rebellion in details (Male)"},
		{Value: "Emilien", Label: "Aimieran", Description: "Romantic French big brother (Male)"},
		{Value: "Andre", Label: "Andelei", Description: "Magnetic, natural, and steady male voice (Male)"},
		{Value: "Radio Gol", Label: "Radio Gol", Description: "Football poet-style commentary (Male)"},
		{Value: "Jada", Label: "Shanghai - Azhen", Description: "Bustling Shanghai sister (Female)"},
		{Value: "Dylan", Label: "Beijing - Xiaodong", Description: "Young man who grew up in Beijing hutongs (Male)"},
		{Value: "Li", Label: "Nanjing - Laoli", Description: "Patient yoga instructor (Male)"},
		{Value: "Marcus", Label: "Shaanxi - Qinchuan", Description: "Full of Shaanxi flavor (Male)"},
		{Value: "Roy", Label: "Minnan - Ajie", Description: "Witty and straightforward Taiwanese brother (Male)"},
		{Value: "Peter", Label: "Tianjin - Lipeter", Description: "Professional crosstalk supporting role from Tianjin (Male)"},
		{Value: "Sunny", Label: "Sichuan - Qinger", Description: "Sichuan girl sweet to your heart (Female)"},
		{Value: "Eric", Label: "Sichuan - Chengcuan", Description: "Lively Chengdu man from the streets (Male)"},
		{Value: "Rocky", Label: "Cantonese - Aqiang", Description: "Humorous and witty Ah Qiang (Male)"},
		{Value: "Kiki", Label: "Cantonese - Aqing", Description: "Sweet Hong Kong girl bestie (Female)"},
	},

	// Tongyi Qwen-TTS series (qwen-tts / qwen-tts-latest / qwen-tts-2025-xx-xx)
	"qwen-tts": {
		{Value: "Cherry", Label: "Qianyue", Description: "Sunny, positive, and naturally friendly young lady (Female)"},
		{Value: "Serena", Label: "Suyao", Description: "Gentle young lady (Female)"},
		{Value: "Ethan", Label: "Chenxu", Description: "Standard Mandarin with some Northern accent, sunny, warm, and energetic (Male)"},
		{Value: "Chelsie", Label: "Qianxue", Description: "Anime virtual girlfriend (Female)"},
		{Value: "Momo", Label: "Motu", Description: "Playful and quirky, cheering you up (Female)"},
		// Other voices can be added as needed
	},
}

// normalizeModel normalizes specific model names to model family keys
// e.g.: qwen3-tts-flash-2025-11-27 -> qwen3-tts-flash
//
//	qwen-tts-2025-05-22       -> qwen-tts
func normalizeModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if strings.HasPrefix(model, "qwen3-tts-flash") {
		return "qwen3-tts-flash"
	}
	if strings.HasPrefix(model, "qwen-tts") {
		return "qwen-tts"
	}
	return model
}

// GetVoicesByModel gets the supported voice list by model name
func GetVoicesByModel(model string) []VoiceInfo {
	key := normalizeModel(model)
	if voices, ok := ModelVoiceMap[key]; ok {
		return voices
	}
	return nil
}

// IsVoiceSupported checks if a specific model supports a certain voice
func IsVoiceSupported(model, voice string) bool {
	if voice == "" {
		return false
	}
	for _, v := range GetVoicesByModel(model) {
		if v.Value == voice {
			return true
		}
	}
	return false
}
