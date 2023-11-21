package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
)

func FixLanguagesScript(vsapiClient *vsapi.Client) {

	file := GetParam(2, "Enter file with vxitems: ")
	jsonFile, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	items := []*VxItem{}
	err = json.Unmarshal(jsonFile, &items)
	if err != nil {
		panic(err)
	}

	for _, item := range items {
		parts := strings.Split(item.Title, "_")
		language := parts[len(parts)-1]
		threeLetter := ""
		if language == "en" {
			threeLetter = "eng"
		} else if language == "nb" {
			threeLetter = "nor"
		} else if language == "zxx" {
			threeLetter = "zxx"
		} else if language == "da" {
			threeLetter = "dan"
		} else if language == "de" {
			threeLetter = "deu"
		} else {
			panic("unknown language: " + language)
		}

		err = vsapiClient.SetItemMetadataField(item.VXID, vscommon.FieldLangsToExport.Value, threeLetter)
		if err != nil {
			panic(err)
		}

		fmt.Printf("VXID: %s, Language: %s, ThreeLetter: %s\n", item.VXID, language, threeLetter)
	}
}
