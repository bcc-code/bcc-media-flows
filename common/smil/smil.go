package smil

import "encoding/xml"

type Smil struct {
	XMLName xml.Name `xml:"smil"`
	Head    Head     `xml:"head"`
	Body    Body     `xml:"body"`
}

type Head struct {
	Meta Meta `xml:"meta"`
}

type Meta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

type Body struct {
	Switch Switch `xml:"switch"`
}

type Switch struct {
	Videos      []Video      `xml:"video"`
	TextStreams []TextStream `xml:"textstream"`
}

type Video struct {
	Src            string `xml:"src,attr"`
	IncludeAudio   string `xml:"includeAudio,attr"`
	SystemLanguage string `xml:"systemLanguage,attr"`
	AudioName      string `xml:"audioName,attr"`
}

type TextStream struct {
	Src            string `xml:"src,attr"`
	SystemLanguage string `xml:"systemLanguage,attr"`
	SubtitleName   string `xml:"subtitleName,attr"`
}
