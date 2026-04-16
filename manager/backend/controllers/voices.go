package controllers

import "strings"

// VoiceInfo describes a Qwen TTS voice
type VoiceInfo struct {
	Value       string   `json:"value"`       // API voice parameter, e.g. "Cherry"
	Label       string   `json:"label"`       // Display name, e.g. "芊悦"
	Description string   `json:"description"` // Short description
	Languages   []string `json:"languages"`   // Supported languages
}

// ModelVoiceMap maps model family -> supported voice list
// Note: Grouped by model "family", e.g. qwen3-tts-flash* as one family, qwen-tts* as another.
var ModelVoiceMap = map[string][]VoiceInfo{
	// Tongyi Qwen3-TTS-Flash series (qwen3-tts-flash / qwen3-tts-flash-2025-11-27 / qwen3-tts-flash-2025-09-18)
	"qwen3-tts-flash": {
		{Value: "Cherry", Label: "芊悦", Description: "Sunny, positive, and naturally friendly young lady (Female)"},
		{Value: "Serena", Label: "苏瑶", Description: "Gentle young lady (Female)"},
		{Value: "Ethan", Label: "晨煦", Description: "Standard Mandarin with some Northern accent, sunny, warm, and energetic (Male)"},
		{Value: "Chelsie", Label: "千雪", Description: "Anime virtual girlfriend (Female)"},
		{Value: "Momo", Label: "茉兔", Description: "Playful and quirky, cheering you up (Female)"},
		{Value: "Vivian", Label: "十三", Description: "Cool and cute with a bit of temper (Female)"},
		{Value: "Moon", Label: "月白", Description: "Straightforward and handsome Yuebai (Male)"},
		{Value: "Maia", Label: "四月", Description: "A blend of intellect and gentleness (Female)"},
		{Value: "Kai", Label: "凯", Description: "An ear SPA experience (Male)"},
		{Value: "Nofish", Label: "不吃鱼", Description: "Designer who can't pronounce retroflex sounds (Male)"},
		{Value: "Bella", Label: "萌宝", Description: "Little loli who doesn't do drunken boxing (Female)"},
		{Value: "Jennifer", Label: "詹妮弗", Description: "Premium brand-quality American female voice with cinematic texture (Female)"},
		{Value: "Ryan", Label: "甜茶", Description: "Full rhythm, explosive acting, dancing between reality and tension (Male)"},
		{Value: "Katerina", Label: "卡捷琳娜", Description: "Mature lady voice with memorable rhythm (Female)"},
		{Value: "Aiden", Label: "艾登", Description: "American big boy who excels at cooking (Male)"},
		{Value: "Eldric Sage", Label: "沧明子", Description: "Steady and wise elder, weathered like pine yet clear as mirror (Male)"},
		{Value: "Mia", Label: "乖小妹", Description: "Gentle as spring water, obedient as first snow (Female)"},
		{Value: "Mochi", Label: "沙小弥", Description: "Clever and bright little adult, childlike yet precocious as Zen (Male)"},
		{Value: "Bellona", Label: "燕铮莺", Description: "Resonant voice, clear articulation, vivid character (Female)"},
		{Value: "Vincent", Label: "田叔", Description: "Hoarse smoky voice telling tales of armies and rivers and lakes (Male)"},
		{Value: "Bunny", Label: "萌小姬", Description: "\"Moe\" attribute bursting little loli (Female)"},
		{Value: "Neil", Label: "阿闻", Description: "Most professional news anchor (Male)"},
		{Value: "Elias", Label: "墨讲师", Description: "Rigorous yet narrative lecturer voice (Female)"},
		{Value: "Arthur", Label: "徐大爷", Description: "Simple voice soaked by years and dry tobacco (Male)"},
		{Value: "Nini", Label: "邻家妹妹", Description: "Voice as soft and sticky as mochi (Female)"},
		{Value: "Ebona", Label: "诡婆婆", Description: "Slightly horror-style grandmother voice (Female)"},
		{Value: "Seren", Label: "小婉", Description: "Gentle and soothing, sleep-aid voice (Female)"},
		{Value: "Pip", Label: "顽屁小孩", Description: "Naughty and mischievous yet full of childlike innocence (Male)"},
		{Value: "Stella", Label: "少女阿月", Description: "Usually sweet to the point of cloying, full of justice when it matters (Female)"},
		{Value: "Bodega", Label: "博德加", Description: "Passionate Spanish uncle (Male)"},
		{Value: "Sonrisa", Label: "索尼莎", Description: "Warm and cheerful Latina sister (Female)"},
		{Value: "Alek", Label: "阿列克", Description: "Warm voice beneath the cold exterior of a fighting nation (Male)"},
		{Value: "Dolce", Label: "多尔切", Description: "Laid-back Italian uncle (Male)"},
		{Value: "Sohee", Label: "素熙", Description: "Gentle, cheerful, and emotionally rich Korean sister (Female)"},
		{Value: "Ono Anna", Label: "小野杏", Description: "Witty and quirky childhood friend (Female)"},
		{Value: "Lenn", Label: "莱恩", Description: "German youth with rationality as base and rebellion in details (Male)"},
		{Value: "Emilien", Label: "埃米尔安", Description: "Romantic French big brother (Male)"},
		{Value: "Andre", Label: "安德雷", Description: "Magnetic, natural, and steady male voice (Male)"},
		{Value: "Radio Gol", Label: "拉迪奥·戈尔", Description: "Football poet-style commentary (Male)"},
		{Value: "Jada", Label: "上海-阿珍", Description: "Bustling Shanghai sister (Female)"},
		{Value: "Dylan", Label: "北京-晓东", Description: "Young man who grew up in Beijing hutongs (Male)"},
		{Value: "Li", Label: "南京-老李", Description: "Patient yoga instructor (Male)"},
		{Value: "Marcus", Label: "陕西-秦川", Description: "Full of Shaanxi flavor (Male)"},
		{Value: "Roy", Label: "闽南-阿杰", Description: "Witty and straightforward Taiwanese brother (Male)"},
		{Value: "Peter", Label: "天津-李彼得", Description: "Professional crosstalk supporting role from Tianjin (Male)"},
		{Value: "Sunny", Label: "四川-晴儿", Description: "Sichuan girl sweet to your heart (Female)"},
		{Value: "Eric", Label: "四川-程川", Description: "Lively Chengdu man from the streets (Male)"},
		{Value: "Rocky", Label: "粤语-阿强", Description: "Humorous and witty Ah Qiang (Male)"},
		{Value: "Kiki", Label: "粤语-阿清", Description: "Sweet Hong Kong girl bestie (Female)"},
	},

	// Tongyi Qwen-TTS series (qwen-tts / qwen-tts-latest / qwen-tts-2025-xx-xx)
	"qwen-tts": {
		{Value: "Cherry", Label: "芊悦", Description: "Sunny, positive, and naturally friendly young lady (Female)"},
		{Value: "Serena", Label: "苏瑶", Description: "Gentle young lady (Female)"},
		{Value: "Ethan", Label: "晨煦", Description: "Standard Mandarin with some Northern accent, sunny, warm, and energetic (Male)"},
		{Value: "Chelsie", Label: "千雪", Description: "Anime virtual girlfriend (Female)"},
		{Value: "Momo", Label: "茉兔", Description: "Playful and quirky, cheering you up (Female)"},
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
