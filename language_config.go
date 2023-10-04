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
}

type LanguageList []Language

var (
	LanguagesByNumber map[int]Language
	LanguagesByISO    map[string]Language
	LanguagesByMU1    map[int]Language
	LanguagesByMU2    map[int]Language
	LanguagesByReaper map[int]Language
)

func init() {
	LanguagesByNumber = languages.ByNumber()
	LanguagesByISO = languages.ByISO6391()
	LanguagesByMU1 = languages.ByMU1()
	LanguagesByMU2 = languages.ByMU2()
	LanguagesByReaper = languages.ByReaperChan()
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
		RelatedMBFieldID:   "portal_mf184670",
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
		RelatedMBFieldID:   "portal_mf31408",
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
		RelatedMBFieldID:   "portal_mf93393",
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
		RelatedMBFieldID:   "portal_mf442906",
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
		RelatedMBFieldID:   "portal_mf903178",
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
		RelatedMBFieldID:   "portal_mf831437",
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
		RelatedMBFieldID:   "portal_mf683496",
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
		RelatedMBFieldID:   "portal_mf307547",
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
		RelatedMBFieldID:   "portal_mf736581",
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
		RelatedMBFieldID:   "portal_mf319181",
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
		RelatedMBFieldID:   "portal_mf351607",
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
		RelatedMBFieldID:   "portal_mf299396",
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
		RelatedMBFieldID:   "portal_mf176737",
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
		RelatedMBFieldID:   "portal_mf122460",
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
		RelatedMBFieldID:   "portal_mf384324",
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
		RelatedMBFieldID:   "portal_mf223187",
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
		RelatedMBFieldID:   "portal_mf890483",
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
		RelatedMBFieldID:   "portal_mf848898",
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
		RelatedMBFieldID:   "portal_mf162929",
	},
	{
		LanguageNumber:     19,
		LanguageName:       "Norsk tolk",
		LanguageNameNative: "Forstår du ikke hva jeg sier?",
		LanguageNameSystem: "Norwegian Translation",
		ISO6391:            "nob",
		ISO6392TwoLetter:   "nb",
		ReaperChannel:      20,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    10,
		MU1ChannelCount:    0,
		MU2ChannelCount:    1,
		RelatedMBFieldID:   "portal_mf961978",
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
		RelatedMBFieldID:   "portal_mf436337",
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
		RelatedMBFieldID:   "portal_mf954584",
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
		RelatedMBFieldID:   "portal_mf855789",
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
		RelatedMBFieldID:   "portal_mf355364",
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
		RelatedMBFieldID:   "portal_mf621489",
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
		RelatedMBFieldID:   "portal_mf447219",
	},
	{
		LanguageNumber:     26,
		LanguageName:       "Afrikansk",
		LanguageNameNative: "",
		LanguageNameSystem: "Afrikaans ",
		ISO6391:            "afr",
		ISO6392TwoLetter:   "af",
		ReaperChannel:      27,
		MU1ChannelStart:    -1,
		MU2ChannelStart:    -1,
		RelatedMBFieldID:   "",
	},
}
