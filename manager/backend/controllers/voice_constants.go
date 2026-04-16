package controllers

import "strings"

// VoiceOption voice option
type VoiceOption struct {
	Value string `json:"value"` // Voice value
	Label string `json:"label"` // Voice display name
}

// VoiceOptions defines voice options for each provider
// Based on Volcano Engine Doubao Speech docs: https://www.volcengine.com/docs/6561/97465
// and Doubao WebSocket docs: https://www.volcengine.com/docs/6561/1257544
var VoiceOptions = map[string][]VoiceOption{
	// Edge TTS voice list (Chinese)
	// Reference: https://blog.csdn.net/u012917925/article/details/134683773
	"edge": {
		{Value: "zh-CN-XiaoxiaoNeural", Label: "Xiaoxiao (Female)"},
		{Value: "zh-CN-YunxiNeural", Label: "Yunxi (Male)"},
		{Value: "zh-CN-YunyangNeural", Label: "Yunyang (Male)"},
		{Value: "zh-CN-XiaoyiNeural", Label: "Xiaoyi (Female)"},
		{Value: "zh-CN-YunjianNeural", Label: "Yunjian (Male)"},
		{Value: "zh-CN-YunxiaNeural", Label: "Yunxia (Male)"},
		{Value: "zh-CN-YunhaoNeural", Label: "Yunhao (Male)"},
		{Value: "zh-CN-XiaohanNeural", Label: "Xiaohan (Female)"},
		{Value: "zh-CN-XiaomoNeural", Label: "Xiaomo (Female)"},
		{Value: "zh-CN-XiaoxuanNeural", Label: "Xiaoxuan (Female)"},
		{Value: "zh-CN-XiaoruiNeural", Label: "Xiaorui (Female)"},
		{Value: "zh-CN-XiaoshuangNeural", Label: "Xiaoshuang (Female)"},
		{Value: "zh-CN-XiaoyanNeural", Label: "Xiaoyan (Female)"},
		{Value: "zh-CN-XiaoyouNeural", Label: "Xiaoyou (Female)"},
		{Value: "zh-CN-XiaozhenNeural", Label: "Xiaozhen (Female)"},
		{Value: "zh-CN-YunfengNeural", Label: "Yunfeng (Male)"},
		{Value: "zh-CN-YunyeNeural", Label: "Yunye (Male)"},
		{Value: "zh-CN-YunzeNeural", Label: "Yunze (Male)"},
	},

	// Microsoft TTS voice list (Chinese)
	"microsoft": {
		{Value: "zh-CN-XiaoxiaoNeural", Label: "Xiaoxiao (Female)"},
		{Value: "zh-CN-YunxiNeural", Label: "Yunxi (Male)"},
		{Value: "zh-CN-YunyangNeural", Label: "Yunyang (Male)"},
		{Value: "zh-CN-XiaoyiNeural", Label: "Xiaoyi (Female)"},
		{Value: "zh-CN-YunjianNeural", Label: "Yunjian (Male)"},
		{Value: "zh-CN-YunxiaNeural", Label: "Yunxia (Male)"},
		{Value: "zh-CN-YunhaoNeural", Label: "Yunhao (Male)"},
		{Value: "zh-CN-XiaohanNeural", Label: "Xiaohan (Female)"},
		{Value: "zh-CN-XiaomoNeural", Label: "Xiaomo (Female)"},
		{Value: "zh-CN-XiaoxuanNeural", Label: "Xiaoxuan (Female)"},
		{Value: "zh-CN-XiaoruiNeural", Label: "Xiaorui (Female)"},
		{Value: "zh-CN-XiaoshuangNeural", Label: "Xiaoshuang (Female)"},
		{Value: "zh-CN-XiaoyanNeural", Label: "Xiaoyan (Female)"},
		{Value: "zh-CN-XiaoyouNeural", Label: "Xiaoyou (Female)"},
		{Value: "zh-CN-XiaozhenNeural", Label: "Xiaozhen (Female)"},
		{Value: "zh-CN-YunfengNeural", Label: "Yunfeng (Male)"},
		{Value: "zh-CN-YunyeNeural", Label: "Yunye (Male)"},
		{Value: "zh-CN-YunzeNeural", Label: "Yunze (Male)"},
	},

	// Doubao TTS voice list (HTTP interface)
	// Reference: https://www.volcengine.com/docs/6561/97465
	"doubao": {
		{Value: "BV700_V2_streaming", Label: "Cancan 2.0"},
		{Value: "BV705_streaming", Label: "Yangyang"},
		{Value: "BV701_V2_streaming", Label: "Qingcang 2.0"},
		{Value: "BV001_V2_streaming", Label: "General Female 2.0"},
		{Value: "BV700_streaming", Label: "Cancan"},
		{Value: "BV406_V2_streaming", Label: "Natural Voice - Zizi 2.0"},
		{Value: "BV406_streaming", Label: "Natural Voice - Zizi"},
		{Value: "BV407_V2_streaming", Label: "Natural Voice - Ranran 2.0"},
		{Value: "BV407_streaming", Label: "Natural Voice - Ranran"},
		{Value: "BV001_streaming", Label: "General Female"},
		{Value: "BV002_streaming", Label: "General Male"},
		{Value: "BV701_streaming", Label: "Qingcang"},
		{Value: "BV119_streaming", Label: "Son-in-law"},
		{Value: "BV102_streaming", Label: "Refined Young Man"},
		{Value: "BV113_streaming", Label: "Sweet Young Lady"},
		{Value: "BV115_streaming", Label: "Classical Young Lady"},
		{Value: "BV007_streaming", Label: "Friendly Female"},
		{Value: "BV056_streaming", Label: "Sunny Male"},
		{Value: "BV005_streaming", Label: "Lively Female"},
		{Value: "BV051_streaming", Label: "Cute Child"},
		{Value: "BV034_streaming", Label: "Intellectual Sister - Bilingual"},
		{Value: "BV033_streaming", Label: "Gentle Young Man"},
		{Value: "BV021_streaming", Label: "Northeastern Buddy"},
		{Value: "BV019_streaming", Label: "Chongqing Guy"},
		{Value: "BV213_streaming", Label: "Guangxi Cousin"},
		{Value: "BV503_streaming", Label: "Energetic Female - Ariana"},
		{Value: "BV504_streaming", Label: "Energetic Male - Jackson"},
		{Value: "BV522_streaming", Label: "Elegant Female"},
		{Value: "BV524_streaming", Label: "Japanese Male"},
		{Value: "BV104_streaming", Label: "Gentle Lady"},
		{Value: "BV004_streaming", Label: "Cheerful Youth"},
		{Value: "BV009_streaming", Label: "Intellectual Female"},
		{Value: "BV008_streaming", Label: "Friendly Male"},
		{Value: "BV064_streaming", Label: "Little Loli"},
		{Value: "BV437_streaming", Label: "Narrator Xiaoshuai - Multi-emotion"},
		{Value: "BV511_streaming", Label: "Lazy Female - Ava"},
		{Value: "BV040_streaming", Label: "Friendly Female - Anna"},
		{Value: "BV138_streaming", Label: "Emotional Female - Lawrence"},
		{Value: "BV704_streaming", Label: "Dialect Cancan"},
		{Value: "BV702_streaming", Label: "Stefan"},
		{Value: "BV421_streaming", Label: "Genius Girl"},
	},

	// Doubao WebSocket TTS voice list
	// Reference official docs "Voice List":
	// https://www.volcengine.com/docs/6561/1257544
	// This maintains commonly used official online voice candidates within the project.
	// Note: Voice list is only for candidate display, no longer strongly bound to model/resource_id by voice name.
	// Actual availability depends on resources enabled in Volcano Console for current appid/access_token.

	"doubao_ws": {
		// Female voices
		{Value: "zh_female_cancan_mars_bigtts", Label: "Cancan / Shiny (Female)"},
		{Value: "zh_female_vv_uranus_bigtts", Label: "Vivi 2.0 (Female)"},
		{Value: "zh_female_vv_jupiter_bigtts", Label: "Vivi O Edition (Female)"},
		{Value: "zh_female_xiaohe_jupiter_bigtts", Label: "Xiaohe O Edition (Female)"},
		{Value: "saturn_zh_female_cancan_tob", Label: "Intellectual Cancan (Female)"},
		{Value: "saturn_zh_female_keainvsheng_tob", Label: "Cute Girl (Female)"},
		{Value: "saturn_zh_female_tiaopigongzhu_tob", Label: "Playful Princess (Female)"},
		{Value: "zh_female_xiaohe_uranus_bigtts", Label: "Xiaohe (Female)"},
		{Value: "zh_female_tianmeitaozi_mars_bigtts", Label: "Sweet Peach (Female)"},
		{Value: "zh_female_wanwanxiaohe_moon_bigtts", Label: "Wanwan Xiaohe (Female)"},
		{Value: "zh_female_qinqienvsheng_moon_bigtts", Label: "Friendly Female (Female)"},
		{Value: "zh_female_vv_mars_bigtts", Label: "Vivi (Female)"},
		{Value: "zh_female_tianmeixiaoyuan_moon_bigtts", Label: "Sweet Xiaoyuan (Female)"},
		{Value: "zh_female_qingchezizi_moon_bigtts", Label: "Clear Zizi (Female)"},
		{Value: "zh_female_kailangjiejie_moon_bigtts", Label: "Cheerful Sister (Female)"},
		{Value: "zh_female_tianmeiyueyue_moon_bigtts", Label: "Sweet Yueyue (Female)"},
		{Value: "zh_female_xinlingjitang_moon_bigtts", Label: "Chicken Soup for the Soul (Female)"},
		{Value: "zh_female_zhixingnvsheng_mars_bigtts", Label: "Intellectual Female (Female)"},
		{Value: "zh_female_wenroushunv_mars_bigtts", Label: "Gentle Lady (Female)"},
		{Value: "zh_female_wenrouxiaoya_moon_bigtts", Label: "Gentle Xiaoya (Female)"},
		{Value: "zh_female_linjianvhai_moon_bigtts", Label: "Girl Next Door (Female)"},
		{Value: "zh_female_shuangkuaisisi_moon_bigtts", Label: "Straightforward Sisi/Skye (Female)"},
		{Value: "zh_female_gaolengyujie_moon_bigtts", Label: "Cold Elegant Lady (Female)"},
		{Value: "zh_female_meilinvyou_moon_bigtts", Label: "Charming Girlfriend (Female)"},
		{Value: "zh_female_sajiaonvyou_moon_bigtts", Label: "Gentle Girlfriend (Coquettish) (Female)"},
		{Value: "zh_female_yuanqinvyou_moon_bigtts", Label: "Coquettish Junior (Female)"},
		{Value: "ICL_zh_female_wenrounvshen_239eff5e8ffa_tob", Label: "Gentle Goddess (Female)"},
		{Value: "ICL_zh_female_chunzhenshaonv_e588402fb8ad_tob", Label: "Innocent Maiden (Female)"},
		{Value: "ICL_zh_female_jinglingxiangdao_1beb294a9e3e_tob", Label: "Elf Guide (Female)"},
		{Value: "ICL_zh_female_yilin_tob", Label: "Caring Sister (Female)"},
		{Value: "ICL_zh_female_chengshujiejie_tob", Label: "Mature Sister (Female)"},
		{Value: "ICL_zh_female_bingjiaojiejie_tob", Label: "Yandere Sister (Female)"},
		{Value: "ICL_zh_female_wumeiyujie_tob", Label: "Charming Lady (Female)"},
		{Value: "ICL_zh_female_aojiaonvyou_tob", Label: "Tsundere Girlfriend (Female)"},
		{Value: "ICL_zh_female_tiexinnvyou_tob", Label: "Caring Girlfriend (Female)"},
		{Value: "ICL_zh_female_xingganyujie_tob", Label: "Sexy Lady (Female)"},
		{Value: "ICL_zh_female_lixingyuanzi_cs_tob", Label: "Rational Yuanzi (Customer Service Female)"},
		{Value: "ICL_zh_female_wuxi_tob", Label: "Energetic Sweet Girl (Female)"},
		{Value: "ICL_zh_female_zhixingwenwan_tob", Label: "Intellectual and Gentle (Female)"},

		// Male voices
		{Value: "saturn_zh_male_shuanglangshaonian_tob", Label: "Cheerful Youth (Male)"},
		{Value: "saturn_zh_male_tiancaitongzhuo_tob", Label: "Genius Deskmate (Male)"},
		{Value: "zh_male_yunzhou_jupiter_bigtts", Label: "Yunzhou O Edition (Male)"},
		{Value: "zh_male_xiaotian_jupiter_bigtts", Label: "Xiaotian O Edition (Male)"},
		{Value: "zh_male_m191_uranus_bigtts", Label: "Yunzhou (Male)"},
		{Value: "zh_male_taocheng_uranus_bigtts", Label: "Xiaotian (Male)"},
		{Value: "en_male_tim_uranus_bigtts", Label: "Tim (English Male)"},
		{Value: "zh_male_yangguangqingnian_moon_bigtts", Label: "Sunny Youth (Male)"},
		{Value: "zh_male_qingshuangnanda_mars_bigtts", Label: "Fresh College Guy (Male)"},
		{Value: "zh_male_wenrouxiaoge_mars_bigtts", Label: "Gentle Young Man (Male)"},
		{Value: "zh_male_qingcang_mars_bigtts", Label: "Qingcang (Male)"},
		{Value: "zh_male_ruyaqingnian_mars_bigtts", Label: "Refined Youth (Male)"},
		{Value: "zh_male_jieshuoxiaoming_moon_bigtts", Label: "Narrator Xiaoming (Male)"},
		{Value: "zh_male_linjiananhai_moon_bigtts", Label: "Boy Next Door (Male)"},
		{Value: "zh_male_yuanboxiaoshu_moon_bigtts", Label: "Knowledgeable Uncle (Male)"},
		{Value: "zh_male_wennuanahu_moon_bigtts", Label: "Warm Ahu/Alvin (Male)"},
		{Value: "zh_male_shaonianzixin_moon_bigtts", Label: "Youth Zixin/Brayan (Male)"},
		{Value: "zh_male_beijingxiaoye_moon_bigtts", Label: "Beijing Young Master (Male)"},
		{Value: "zh_male_jingqiangkanye_moon_bigtts", Label: "Beijing Accent Kanye/Harmony (Male)"},
		{Value: "zh_male_guozhoudege_moon_bigtts", Label: "Guangzhou Brother De (Male)"},
		{Value: "zh_male_haoyuxiaoge_moon_bigtts", Label: "Haoyu Young Man (Male)"},
		{Value: "zh_male_shenyeboke_moon_bigtts", Label: "Late Night Podcast (Male)"},
		{Value: "zh_male_aojiaobazong_moon_bigtts", Label: "Tsundere CEO (Male)"},
		{Value: "zh_male_dongfanghaoran_moon_bigtts", Label: "Dongfang Haoran (Male)"},
		{Value: "zh_male_M100_conversation_wvae_bigtts", Label: "Gentleman/Lucas (Male)"},
		{Value: "zh_male_xudong_conversation_wvae_bigtts", Label: "Happy Xiaodong/Daniel (Male)"},
		{Value: "zh_male_qingyiyuxuan_mars_bigtts", Label: "Sunny Achen (Male)"},
		{Value: "en_male_jason_conversation_wvae_bigtts", Label: "Cheerful Senior (Male)"},
		{Value: "ICL_zh_male_lengkugege_v1_tob", Label: "Cold Brother (Male)"},
		{Value: "ICL_zh_male_shenmi_v1_tob", Label: "Clever Guy (Male)"},
		{Value: "ICL_zh_male_BV705_streaming_cs_tob", Label: "Yangyang (Male)"},
		{Value: "ICL_zh_male_menyoupingxiaoge_ffed9fc2fee7_tob", Label: "Silent Young Man (Male)"},
		{Value: "ICL_zh_male_anrenqinzhu_cd62e63dcdab_tob", Label: "Dark Blade Qin Lord (Male)"},
		{Value: "ICL_zh_male_guaogongzi_v1_tob", Label: "Proud Young Master (Male)"},
		{Value: "ICL_zh_male_bingruogongzi_tob", Label: "Sickly Young Master (Male)"},
		{Value: "ICL_zh_male_bingjiaodidi_tob", Label: "Yandere Younger Brother (Male)"},
		{Value: "ICL_zh_male_aomanshaoye_tob", Label: "Arrogant Young Master (Male)"},
		{Value: "ICL_zh_male_chunzhenxuedi_tob", Label: "Innocent Junior (Male)"},
		{Value: "ICL_zh_male_yourougongzi_tob", Label: "Indecisive Young Master (Male)"},
		{Value: "ICL_zh_male_tiexinnanyou_tob", Label: "Caring Boyfriend (Male)"},
		{Value: "ICL_zh_male_shaonianjiangjun_tob", Label: "Young General (Male)"},
		{Value: "ICL_zh_male_bingjiaogege_tob", Label: "Yandere Brother (Male)"},
		{Value: "ICL_zh_male_xuebanantongzhuo_tob", Label: "Straight-A Male Deskmate (Male)"},
		{Value: "ICL_zh_male_youmoshushu_tob", Label: "Humorous Uncle (Male)"},
		{Value: "ICL_zh_male_wenrounantongzhuo_tob", Label: "Gentle Male Deskmate (Male)"},
		{Value: "ICL_zh_male_youmodaye_tob", Label: "Humorous Old Man (Male)"},
		{Value: "ICL_zh_male_shenmifashi_tob", Label: "Mysterious Mage (Male)"},
		{Value: "ICL_zh_male_lengjunshangsi_tob", Label: "Cold Superior (Male)"},
		{Value: "ICL_en_male_michael_tob", Label: "Michael (American English Male)"},

		// IP/Special voices
		{Value: "zh_male_lubanqihao_mars_bigtts", Label: "Luban No.7 (Male)"},
		{Value: "zh_female_yangmi_mars_bigtts", Label: "Lin Xiao (Female)"},
		{Value: "zh_female_linzhiling_mars_bigtts", Label: "Lingling Sister (Female)"},
		{Value: "zh_female_jiyejizi2_mars_bigtts", Label: "Kasukabe Sister (Female)"},
		{Value: "zh_male_tangseng_mars_bigtts", Label: "Tang Monk (Male)"},
		{Value: "zh_male_zhubajie_mars_bigtts", Label: "Zhu Bajie (Male)"},
		{Value: "zh_female_naying_mars_bigtts", Label: "Straightforward Yingzi (Female)"},
		{Value: "zh_female_leidian_mars_bigtts", Label: "Female Thor (Female)"},
		{Value: "zh_male_sunwukong_mars_bigtts", Label: "Monkey King (Male)"},
		{Value: "zh_male_xionger_mars_bigtts", Label: "Xionger (Male)"},
		{Value: "zh_female_peiqi_mars_bigtts", Label: "Peppa Pig (Female)"},
		{Value: "zh_female_yingtaowanzi_mars_bigtts", Label: "Chibi Maruko (Female)"},
		{Value: "zh_male_silang_mars_bigtts", Label: "Silang (Male)"},
	},

	// Minimax TTS voice list
	// Reference: https://www.minimaxi.com/document/guides/tts-model
	"minimax": {
		// Chinese (Mandarin)
		{Value: "male-qn-qingse", Label: "Youthful Young Man Voice"},
		{Value: "male-qn-jingying", Label: "Elite Young Man Voice"},
		{Value: "male-qn-badao", Label: "Domineering Young Man Voice"},
		{Value: "male-qn-daxuesheng", Label: "College Student Voice"},
		{Value: "female-shaonv", Label: "Young Girl Voice"},
		{Value: "female-yujie", Label: "Mature Lady Voice"},
		{Value: "female-chengshu", Label: "Mature Woman Voice"},
		{Value: "female-tianmei", Label: "Sweet Female Voice"},
		{Value: "male-qn-qingse-jingpin", Label: "Youthful Young Man Voice - Beta"},
		{Value: "male-qn-jingying-jingpin", Label: "Elite Young Man Voice - Beta"},
		{Value: "male-qn-badao-jingpin", Label: "Domineering Young Man Voice - Beta"},
		{Value: "male-qn-daxuesheng-jingpin", Label: "College Student Voice - Beta"},
		{Value: "female-shaonv-jingpin", Label: "Young Girl Voice - Beta"},
		{Value: "female-yujie-jingpin", Label: "Mature Lady Voice - Beta"},
		{Value: "female-chengshu-jingpin", Label: "Mature Woman Voice - Beta"},
		{Value: "female-tianmei-jingpin", Label: "Sweet Female Voice - Beta"},
		{Value: "clever_boy", Label: "Clever Boy"},
		{Value: "cute_boy", Label: "Cute Boy"},
		{Value: "lovely_girl", Label: "Lovely Girl"},
		{Value: "cartoon_pig", Label: "Cartoon Pig Xiaoqi"},
		{Value: "bingjiao_didi", Label: "Yandere Younger Brother"},
		{Value: "junlang_nanyou", Label: "Handsome Boyfriend"},
		{Value: "chunzhen_xuedi", Label: "Innocent Junior"},
		{Value: "lengdan_xiongzhang", Label: "Cold Senior"},
		{Value: "badao_shaoye", Label: "Domineering Young Master"},
		{Value: "tianxin_xiaoling", Label: "Sweetheart Xiaoling"},
		{Value: "qiaopi_mengmei", Label: "Playful Cute Girl"},
		{Value: "wumei_yujie", Label: "Charming Lady"},
		{Value: "diadia_xuemei", Label: "Coquettish Junior"},
		{Value: "danya_xuejie", Label: "Elegant Senior"},
		{Value: "Chinese (Mandarin)_Reliable_Executive", Label: "Steady Executive"},
		{Value: "Chinese (Mandarin)_News_Anchor", Label: "News Anchor Female"},
		{Value: "Chinese (Mandarin)_Mature_Woman", Label: "Tsundere Lady"},
		{Value: "Chinese (Mandarin)_Unrestrained_Young_Man", Label: "Unrestrained Youth"},
		{Value: "Arrogant_Miss", Label: "Arrogant Miss"},
		{Value: "Robot_Armor", Label: "Robot Armor"},
		{Value: "Chinese (Mandarin)_Kind-hearted_Antie", Label: "Kind-hearted Auntie"},
		{Value: "Chinese (Mandarin)_HK_Flight_Attendant", Label: "Hong Kong Flight Attendant"},
		{Value: "Chinese (Mandarin)_Humorous_Elder", Label: "Humorous Elder"},
		{Value: "Chinese (Mandarin)_Gentleman", Label: "Gentle Male Voice"},
		{Value: "Chinese (Mandarin)_Warm_Bestie", Label: "Warm Bestie"},
		{Value: "Chinese (Mandarin)_Male_Announcer", Label: "Male Announcer"},
		{Value: "Chinese (Mandarin)_Sweet_Lady", Label: "Sweet Lady"},
		{Value: "Chinese (Mandarin)_Southern_Young_Man", Label: "Southern Young Man"},
		{Value: "Chinese (Mandarin)_Wise_Women", Label: "Wise Woman"},
		{Value: "Chinese (Mandarin)_Gentle_Youth", Label: "Gentle Youth"},
		{Value: "Chinese (Mandarin)_Warm_Girl", Label: "Warm Girl"},
		{Value: "Chinese (Mandarin)_Kind-hearted_Elder", Label: "Kind-hearted Elder"},
		{Value: "Chinese (Mandarin)_Cute_Spirit", Label: "Cute Spirit"},
		{Value: "Chinese (Mandarin)_Radio_Host", Label: "Radio Host"},
		{Value: "Chinese (Mandarin)_Lyrical_Voice", Label: "Lyrical Male Voice"},
		{Value: "Chinese (Mandarin)_Straightforward_Boy", Label: "Straightforward Boy"},
		{Value: "Chinese (Mandarin)_Sincere_Adult", Label: "Sincere Youth"},
		{Value: "Chinese (Mandarin)_Gentle_Senior", Label: "Gentle Senior"},
		{Value: "Chinese (Mandarin)_Stubborn_Friend", Label: "Stubborn Friend"},
		{Value: "Chinese (Mandarin)_Crisp_Girl", Label: "Crisp Girl"},
		{Value: "Chinese (Mandarin)_Pure-hearted_Boy", Label: "Pure-hearted Boy Next Door"},
		{Value: "Chinese (Mandarin)_Soft_Girl", Label: "Soft Girl"},
		// Chinese (Cantonese)
		{Value: "Cantonese_ProfessionalHost（F)", Label: "Professional Female Host"},
		{Value: "Cantonese_GentleLady", Label: "Gentle Female"},
		{Value: "Cantonese_ProfessionalHost（M)", Label: "Professional Male Host"},
		{Value: "Cantonese_PlayfulMan", Label: "Playful Male"},
		{Value: "Cantonese_CuteGirl", Label: "Cute Girl"},
		{Value: "Cantonese_KindWoman", Label: "Kind Female"},
		// English
		{Value: "Santa_Claus", Label: "Santa Claus"},
		{Value: "Grinch", Label: "Grinch"},
		{Value: "Rudolph", Label: "Rudolph"},
		{Value: "Arnold", Label: "Arnold"},
		{Value: "Charming_Santa", Label: "Charming Santa"},
		{Value: "Charming_Lady", Label: "Charming Lady"},
		{Value: "Sweet_Girl", Label: "Sweet Girl"},
		{Value: "Cute_Elf", Label: "Cute Elf"},
		{Value: "Attractive_Girl", Label: "Attractive Girl"},
		{Value: "Serene_Woman", Label: "Serene Woman"},
		{Value: "English_Trustworthy_Man", Label: "Trustworthy Man"},
		{Value: "English_Graceful_Lady", Label: "Graceful Lady"},
		{Value: "English_Aussie_Bloke", Label: "Aussie Bloke"},
		{Value: "English_Whispering_girl", Label: "Whispering girl"},
		{Value: "English_Diligent_Man", Label: "Diligent Man"},
		{Value: "English_Gentle-voiced_man", Label: "Gentle-voiced man"},
	},

	// Alibaba Cloud Qwen TTS voice list (basic list, model filtering handled by GetAliyunQwenVoicesByModel)
	"aliyun_qwen": {
		{Value: "Cherry", Label: "Qianyue"},
		{Value: "Serena", Label: "Suyao"},
		{Value: "Ethan", Label: "Chenxu"},
		{Value: "Chelsie", Label: "Qianxue"},
		{Value: "Momo", Label: "Motu"},
		{Value: "Vivian", Label: "Shisan"},
		{Value: "Moon", Label: "Yuebai"},
		{Value: "Maia", Label: "Siyue"},
		{Value: "Kai", Label: "Kai"},
		{Value: "Nofish", Label: "No Fish"},
		{Value: "Bella", Label: "Cute Baby"},
		{Value: "Jennifer", Label: "Jennifer"},
		{Value: "Ryan", Label: "Sweet Tea"},
	},

	// iFlytek online TTS voice list
	// Note: A set of commonly used static voices is retained here, final availability depends on actual authorization in iFlytek console.
	// Reference:
	// https://www.xfyun.cn/doc/tts/online_tts/API.html
	// https://aiui.xfyun.cn/doc/aiui/3_access_service/access_interact/functions/speech_synthesis.html
	"xunfei": {
		{Value: "xiaoyan", Label: "Xiaoyan (Female, Default Recommended)"},
		{Value: "xiaofeng", Label: "Xiaofeng (Male)"},
		{Value: "yezi", Label: "Xiaolu (Female)"},
		{Value: "yifei", Label: "Yifei (Female)"},
		{Value: "yiping", Label: "Yiping (Female)"},
		{Value: "qige", Label: "Qige (Male)"},
		{Value: "chaoge", Label: "Chaoge (Male)"},
		{Value: "pengfei", Label: "Xiaopeng (Male)"},
		{Value: "xiaoxin", Label: "Cute Xiaoxin (Child)"},
		{Value: "john", Label: "John (English Male)"},
		{Value: "catherine", Label: "Catherine (English Female)"},
	},

	// iFlytek super realistic TTS voice list
	// Note: A set of recommended static voices is retained, final availability depends on iFlytek console authorization.
	"xunfei_super_tts": {
		{Value: "x6_lingxiaoxue_pro", Label: "Ling Xiaoxue (x6)"},
		{Value: "x6_lingfeiyi_pro", Label: "Ling Feiyi (x6)"},
		{Value: "x6_lingxiaoli_pro", Label: "Ling Xiaoli (x6)"},
		{Value: "x6_lingxiaoyue_pro", Label: "Ling Xiaoyue (x6)"},
		{Value: "x6_lingxiaoxuan_pro", Label: "Ling Xiaoxuan (x6)"},
		{Value: "x6_lingyuyan_pro", Label: "Ling Yuyan (x6)"},
		{Value: "x6_lingyouyou_pro", Label: "Ling Youyou (x6)"},
		{Value: "x6_feizheChat_pro", Label: "Fei Zhe Chat (x6)"},
		{Value: "x6_xiaoqiChat_pro", Label: "Xiaoqi Chat (x6)"},
		{Value: "x5_lingxiaotang_flow", Label: "Ling Xiaotang (x5)"},
		{Value: "x5_lingyuzhao_flow", Label: "Ling Yuzhao (x5)"},
		{Value: "x4_zijin_oral", Label: "Zijin (x4, Colloquial)"},
		{Value: "x4_ziyang_oral", Label: "Ziyang (x4, Colloquial)"},
	},

	// Zhipu TTS voice list
	"zhipu": {
		{Value: "tongtong", Label: "Tongtong (Default Voice)"},
		{Value: "chuichui", Label: "Chuichui"},
		{Value: "xiaochen", Label: "Xiaochen"},
		{Value: "jam", Label: "Dongdong Animal Circle Jam Voice"},
		{Value: "kazi", Label: "Dongdong Animal Circle Kazi Voice"},
		{Value: "douji", Label: "Dongdong Animal Circle Douji Voice"},
		{Value: "luodo", Label: "Dongdong Animal Circle Luodo Voice"},
	},
}

// GetVoiceOptionsByProvider gets voice list by provider
func GetVoiceOptionsByProvider(provider string) []VoiceOption {
	if voices, ok := VoiceOptions[provider]; ok {
		return voices
	}
	return []VoiceOption{}
}

// GetAliyunQwenVoicesByModel gets voice list by Qwen model name
// Uses model mapping in qwen package to get accurate voice list
func GetAliyunQwenVoicesByModel(model string) []VoiceOption {
	model = strings.TrimSpace(model)
	if model == "" {
		// 如果没有模型，返回基础列表
		return GetVoiceOptionsByProvider("aliyun_qwen")
	}

	// 使用本地函数获取模型对应的音色列表
	voices := GetVoicesByModel(model)
	if voices == nil || len(voices) == 0 {
		// 如果找不到对应模型的音色，返回基础列表
		return GetVoiceOptionsByProvider("aliyun_qwen")
	}

	// 将 VoiceInfo 转换为 VoiceOption
	result := make([]VoiceOption, 0, len(voices))
	for _, v := range voices {
		result = append(result, VoiceOption{
			Value: v.Value,
			Label: v.Label,
		})
	}
	return result
}
