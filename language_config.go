package bccmflows

type Language struct {
	LanguageNumber     int
	LanguageName       string
	LanguageNameNative string
	LanguageNameSystem string
	ISO6391            string
	ISO6392TwoLetter   string
	ReaperChannel      int
	MU1ChannelStart    int
	MU1ChannelCount    int
	MU2ChannelStart    int
	MU2ChannelCount    int
	RelatedMBFieldID   string
	SoftronStartCh     int
	MBPreviewTag       string
}

type LanguageList []Language

var (
	LanguagesByNumber       map[int]Language
	LanguagesByISO          map[string]Language
	LanguagesByMU1          map[int]Language
	LanguagesByMU2          map[int]Language
	LanguagesByReaper       map[int]Language
	LanguagesByISOTwoLetter map[string]Language
	LanguageBySoftron       map[int]Language
)

func init() {
	LanguagesByNumber = languages.ByNumber()
	LanguagesByISO = languages.ByISO6391()
	LanguagesByMU1 = languages.ByMU1()
	LanguagesByMU2 = languages.ByMU2()
	LanguagesByReaper = languages.ByReaperChan()
	LanguagesByISOTwoLetter = languages.ByISO6392TwoLetter()
	LanguageBySoftron = languages.BySoftron()
}

func (l LanguageList) BySoftron() map[int]Language {
	out := make(map[int]Language)
	for _, lang := range l {
		if lang.SoftronStartCh < 0 {
			continue
		}
		out[lang.SoftronStartCh] = lang
	}
	return out
}

func (l LanguageList) ByNumber() map[int]Language {
	out := make(map[int]Language)
	for _, lang := range l {
		out[lang.LanguageNumber] = lang
	}
	return out
}

func (l LanguageList) ByISO6391() map[string]Language {
	out := make(map[string]Language)
	for _, lang := range l {
		out[lang.ISO6391] = lang
	}
	return out
}

func (l LanguageList) ByMU1() map[int]Language {
	out := make(map[int]Language)
	for _, lang := range l {
		if lang.MU1ChannelStart < 0 {
			continue
		}

		for i := 0; i < lang.MU1ChannelCount; i++ {
			out[lang.MU1ChannelStart+i] = lang
		}
	}
	return out
}

func (l LanguageList) ByMU2() map[int]Language {
	out := make(map[int]Language)
	for _, lang := range l {
		if lang.MU2ChannelStart < 0 {
			continue
		}
		for i := 0; i < lang.MU2ChannelCount; i++ {
			out[lang.MU2ChannelStart+i] = lang
		}
	}
	return out
}

func (l LanguageList) ByReaperChan() map[int]Language {
	out := make(map[int]Language)
	for _, lang := range l {
		if lang.ReaperChannel < 0 {
			continue
		}
		out[lang.ReaperChannel] = lang
	}
	return out
}

func (l LanguageList) ByISO6392TwoLetter() map[string]Language {
	out := make(map[string]Language)
	for _, lang := range l {
		out[lang.ISO6392TwoLetter] = lang
	}
	return out
}

func (l LanguageList) Len() int {
	return len(l)
}

func (l LanguageList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l LanguageList) Less(i, j int) bool {
	return l[i].LanguageNumber < l[j].LanguageNumber
}

// Master: https://www.notion.so/bccmedia/Language-codes-222bc3bd6240428a93d03a84761ab57b?pvs=4
var languages = LanguageList{
	{
		LanguageNumber:     0,
		LanguageName:       "Norsk",
		LanguageNameNative: "Norsk",
		LanguageNameSystem: "Norwegian",
		ISO6391:            "nor",
		ISO6392TwoLetter:   "no",
		ReaperChannel:      1,
		MU1ChannelStart:    1,
		MU2ChannelStart:    1,
		MU1ChannelCount:    2,
		MU2ChannelCount:    2,
		SoftronStartCh:     0,
		RelatedMBFieldID:   "portal_mf184670",
		MBPreviewTag:       "mul_nor_low",
	},
	{
		LanguageNumber:     1,
		LanguageName:       "Tysk",
		LanguageNameNative: "Deutch",
		LanguageNameSystem: "German",
		ISO6391:            "deu",
		ISO6392TwoLetter:   "de",
		ReaperChannel:      2,
		MU1ChannelStart:    3,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    2,
		MU2ChannelCount:    0,
		SoftronStartCh:     2,
		RelatedMBFieldID:   "portal_mf31408",
		MBPreviewTag:       "mul_deu_low",
	},
	{
		LanguageNumber:     2,
		LanguageName:       "Hollandsk",
		LanguageNameNative: "Nederland",
		LanguageNameSystem: "Dutch",
		ISO6391:            "nld",
		ISO6392TwoLetter:   "nl",
		ReaperChannel:      3,
		MU1ChannelStart:    5,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    2,
		MU2ChannelCount:    0,
		SoftronStartCh:     4,
		RelatedMBFieldID:   "portal_mf93393",
		MBPreviewTag:       "mul_nld_low",
	},
	{
		LanguageNumber:     3,
		LanguageName:       "Engelsk",
		LanguageNameNative: "English",
		LanguageNameSystem: "English",
		ISO6391:            "eng",
		ISO6392TwoLetter:   "en",
		ReaperChannel:      4,
		MU1ChannelStart:    7,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    2,
		MU2ChannelCount:    0,
		SoftronStartCh:     6,
		RelatedMBFieldID:   "portal_mf442906",
		MBPreviewTag:       "mul_eng_low",
	},
	{
		LanguageNumber:     4,
		LanguageName:       "Fransk",
		LanguageNameNative: "Français",
		LanguageNameSystem: "French",
		ISO6391:            "fra",
		ISO6392TwoLetter:   "fr",
		ReaperChannel:      5,
		MU1ChannelStart:    9,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    1,
		MU2ChannelCount:    0,
		SoftronStartCh:     8,
		RelatedMBFieldID:   "portal_mf903178",
		MBPreviewTag:       "mul_fra_low",
	},
	{
		LanguageNumber:     5,
		LanguageName:       "Spansk",
		LanguageNameNative: "Española",
		LanguageNameSystem: "Spanish",
		ISO6391:            "spa",
		ISO6392TwoLetter:   "es",
		ReaperChannel:      6,
		MU1ChannelStart:    10,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    1,
		MU2ChannelCount:    0,
		SoftronStartCh:     10,
		RelatedMBFieldID:   "portal_mf831437",
		MBPreviewTag:       "mul_spa_low",
	},
	{
		LanguageNumber:     6,
		LanguageName:       "Finsk",
		LanguageNameNative: "Suomalainen",
		LanguageNameSystem: "Finnish",
		ISO6391:            "fin",
		ISO6392TwoLetter:   "fi",
		ReaperChannel:      7,
		MU1ChannelStart:    11,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    1,
		MU2ChannelCount:    0,
		SoftronStartCh:     12,
		RelatedMBFieldID:   "portal_mf683496",
		MBPreviewTag:       "mul_fin_low",
	},
	{
		LanguageNumber:     7,
		LanguageName:       "Russisk",
		LanguageNameNative: "Русский",
		LanguageNameSystem: "Russian",
		ISO6391:            "rus",
		ISO6392TwoLetter:   "ru",
		ReaperChannel:      8,
		MU1ChannelStart:    12,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    1,
		MU2ChannelCount:    0,
		SoftronStartCh:     14,
		RelatedMBFieldID:   "portal_mf307547",
		MBPreviewTag:       "mul_rus_low",
	},
	{
		LanguageNumber:     8,
		LanguageName:       "Portugisisk",
		LanguageNameNative: "Português",
		LanguageNameSystem: "Portuguese",
		ISO6391:            "por",
		ISO6392TwoLetter:   "pt",
		ReaperChannel:      9,
		MU1ChannelStart:    13,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    1,
		MU2ChannelCount:    0,
		SoftronStartCh:     16,
		RelatedMBFieldID:   "portal_mf736581",
		MBPreviewTag:       "mul_por_low",
	},
	{
		LanguageNumber:     9,
		LanguageName:       "Rumensk",
		LanguageNameNative: "Română",
		LanguageNameSystem: "Romanian",
		ISO6391:            "ron",
		ISO6392TwoLetter:   "ro",
		ReaperChannel:      10,
		MU1ChannelStart:    14,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    1,
		MU2ChannelCount:    0,
		SoftronStartCh:     18,
		RelatedMBFieldID:   "portal_mf319181",
		MBPreviewTag:       "mul_ron_low",
	},
	{
		LanguageNumber:     10,
		LanguageName:       "Tyrkisk",
		LanguageNameNative: "Türkçe",
		LanguageNameSystem: "Turkish",
		ISO6391:            "tur",
		ISO6392TwoLetter:   "tr",
		ReaperChannel:      11,
		MU1ChannelStart:    15,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    1,
		MU2ChannelCount:    0,
		SoftronStartCh:     20,
		RelatedMBFieldID:   "portal_mf351607",
		MBPreviewTag:       "mul_tur_low",
	},
	{
		LanguageNumber:     11,
		LanguageName:       "Polsk",
		LanguageNameNative: "Polski",
		LanguageNameSystem: "Polish",
		ISO6391:            "pol",
		ISO6392TwoLetter:   "pl",
		ReaperChannel:      12,
		MU1ChannelStart:    16,
		MU2ChannelStart:    -1,
		MU1ChannelCount:    1,
		MU2ChannelCount:    0,
		SoftronStartCh:     22,
		RelatedMBFieldID:   "portal_mf299396",
		MBPreviewTag:       "mul_pol_low",
	},
	{
		LanguageNumber:     12,
		LanguageName:       "Bulgarsk",
		LanguageNameNative: "български",
		LanguageNameSystem: "Bulgarian",
		ISO6391:            "bul",
		ISO6392TwoLetter:   "bg",
		ReaperChannel:      13,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    3,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		SoftronStartCh:     24,
		RelatedMBFieldID:   "portal_mf176737",
		MBPreviewTag:       "mul_bul_low",
	},
	{
		LanguageNumber:     13,
		LanguageName:       "Ungarsk",
		LanguageNameNative: "Magyar",
		LanguageNameSystem: "Hungarian",
		ISO6391:            "hun",
		ISO6392TwoLetter:   "hu",
		ReaperChannel:      14,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    4,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		SoftronStartCh:     26,
		RelatedMBFieldID:   "portal_mf122460",
		MBPreviewTag:       "mul_hun_low",
	},
	{
		LanguageNumber:     14,
		LanguageName:       "Italiensk",
		LanguageNameNative: "Italiano",
		LanguageNameSystem: "Italian",
		ISO6391:            "ita",
		ISO6392TwoLetter:   "it",
		ReaperChannel:      15,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    5,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		SoftronStartCh:     28,
		RelatedMBFieldID:   "portal_mf384324",
		MBPreviewTag:       "mul_ita_low",
	},
	{
		LanguageNumber:     15,
		LanguageName:       "Slovensk",
		LanguageNameNative: "Slovenščina",
		LanguageNameSystem: "Slovenian",
		ISO6391:            "slv",
		ISO6392TwoLetter:   "sl",
		ReaperChannel:      16,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    6,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		SoftronStartCh:     30,
		RelatedMBFieldID:   "portal_mf223187",
		MBPreviewTag:       "mul_slv_low",
	},
	{
		LanguageNumber:     16,
		LanguageName:       "Kinesisk",
		LanguageNameNative: "简体中文",
		LanguageNameSystem: "Simplified Chinese",
		ISO6391:            "cmn",
		ISO6392TwoLetter:   "zh",
		ReaperChannel:      17,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    7,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		SoftronStartCh:     32,
		RelatedMBFieldID:   "portal_mf890483",
		MBPreviewTag:       "mul_cmn_low",
	},
	{
		LanguageNumber:     17,
		LanguageName:       "Kroatisk",
		LanguageNameNative: "Hrvatski",
		LanguageNameSystem: "Croatian",
		ISO6391:            "hrv",
		ISO6392TwoLetter:   "hr",
		ReaperChannel:      18,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    8,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		SoftronStartCh:     34,
		RelatedMBFieldID:   "portal_mf848898",
		MBPreviewTag:       "mul_hrv_low",
	},
	{
		LanguageNumber:     18,
		LanguageName:       "Dansk",
		LanguageNameNative: "Dansk",
		LanguageNameSystem: "Danish",
		ISO6391:            "dan",
		ISO6392TwoLetter:   "da",
		ReaperChannel:      19,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    9,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		SoftronStartCh:     36,
		RelatedMBFieldID:   "portal_mf162929",
		MBPreviewTag:       "mul_dan_low",
	},
	{
		LanguageNumber:     19,
		LanguageName:       "Norsk tolk",
		LanguageNameNative: "Forstår du ikke hva jeg sier?",
		LanguageNameSystem: "Norwegian Translation",
		ISO6391:            "nob",
		ISO6392TwoLetter:   "no-x-tolk",
		ReaperChannel:      20,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    10,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		SoftronStartCh:     38,
		RelatedMBFieldID:   "portal_mf961978",
		MBPreviewTag:       "mul_nob_low",
	},
	{
		LanguageNumber:     20,
		LanguageName:       "Tradisjonell kinesisk",
		LanguageNameNative: "繁體中文",
		LanguageNameSystem: "Traditional Chinese",
		ISO6391:            "yue",
		ISO6392TwoLetter:   "",
		ReaperChannel:      21,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    11,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		SoftronStartCh:     40,
		RelatedMBFieldID:   "portal_mf436337",
		MBPreviewTag:       "mul_yue_low",
	},
	{
		LanguageNumber:     21,
		LanguageName:       "Maylaisisk",
		LanguageNameNative: "മലയാളം",
		LanguageNameSystem: "Malayallam",
		ISO6391:            "mal",
		ISO6392TwoLetter:   "",
		ReaperChannel:      22,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    -1,
		SoftronStartCh:     42,
		RelatedMBFieldID:   "portal_mf954584",
		MBPreviewTag:       "mul_mal_low",
	},
	{
		LanguageNumber:     22,
		LanguageName:       "Tamil",
		LanguageNameNative: "தமிழ்",
		LanguageNameSystem: "Tamil",
		ISO6391:            "tam",
		ISO6392TwoLetter:   "ta",
		ReaperChannel:      23,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    -1,
		SoftronStartCh:     44,
		RelatedMBFieldID:   "portal_mf855789",
		MBPreviewTag:       "mul_tam_low",
	},
	{
		LanguageNumber:     23,
		LanguageName:       "Estisk",
		LanguageNameNative: "eesti keel",
		LanguageNameSystem: "Estonian",
		ISO6391:            "est",
		ISO6392TwoLetter:   "",
		ReaperChannel:      24,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    -1,
		SoftronStartCh:     48,
		RelatedMBFieldID:   "portal_mf355364",
		MBPreviewTag:       "mul_est_low",
	},
	{
		LanguageNumber:     24,
		LanguageName:       "Khasi",
		LanguageNameNative: "Khasi",
		LanguageNameSystem: "Khasi",
		ISO6391:            "kha",
		ISO6392TwoLetter:   "",
		ReaperChannel:      25,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    -1,
		SoftronStartCh:     50,
		RelatedMBFieldID:   "portal_mf621489",
		MBPreviewTag:       "mul_kha_low",
	},
	{
		LanguageNumber:     25,
		LanguageName:       "Swahili",
		LanguageNameNative: "",
		LanguageNameSystem: "Swahili",
		ISO6391:            "swa",
		ISO6392TwoLetter:   "",
		ReaperChannel:      26,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    -1,
		SoftronStartCh:     52,
		RelatedMBFieldID:   "portal_mf447219",
		MBPreviewTag:       "mul_swa_low",
	},
	{
		LanguageNumber:     26,
		LanguageName:       "Afrikaans",
		LanguageNameNative: "",
		LanguageNameSystem: "Afrikaans",
		ISO6391:            "afr",
		ISO6392TwoLetter:   "af",
		ReaperChannel:      27,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    -1,
		RelatedMBFieldID:   "",
		SoftronStartCh:     54,
		MBPreviewTag:       "mul_afr_low",
	},
	{
		LanguageNumber:     99,
		LanguageName:       "No linguistic content",
		LanguageNameNative: "",
		LanguageNameSystem: "No linguistic content",
		ISO6391:            "zxx",
		ISO6392TwoLetter:   "zxx",
		ReaperChannel:      -1,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    -1,
		RelatedMBFieldID:   "",
		SoftronStartCh:     -1,
		MBPreviewTag:       "",
	},
	{
		LanguageNumber:     100,
		LanguageName:       "AI Generated",
		LanguageNameNative: "AI Generated",
		LanguageNameSystem: "AI Generated",
		ISO6391:            "und",
		ISO6392TwoLetter:   "und-x-ai-generated",
		ReaperChannel:      -2,
		MU1ChannelStart:    -2,
		MU2ChannelStart:    -2,
		RelatedMBFieldID:   "",
		SoftronStartCh:     -1,
		MBPreviewTag:       "",
	},
}
