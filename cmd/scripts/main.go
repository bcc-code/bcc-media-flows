package main

import (
	"fmt"
	"os"

	"github.com/bcc-code/bccm-flows/cmd/scripts/scripts"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	_ "github.com/lib/pq"
)

func main() {

	vsapiClient := vsapi.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))

	scriptFuncs := map[string]func(){
		"list-collection": func() {
			scripts.GetVxItemsFromCollectionScript(vsapiClient)
		},
		"trigger-items": func() {
			scripts.TriggerItemsScript()
		},
		"get-tracks": func() {
			scripts.GetBmmTracksFromVXItemsScript()
		},
		"import-tracks": func() {
			scripts.ImportBmmTracksAsVODEpisodesScript()
		},
		"fix-languages": func() {
			scripts.FixLanguagesScript(vsapiClient)
		},
		"link-assets": func() {
			scripts.LinkAssetsScript()
		},
	}

	fmt.Printf("Scripts:\n")
	for script, _ := range scriptFuncs {
		fmt.Printf("%s\n", script)
	}
	fmt.Printf("\n")

	script := scripts.GetParam(1, "Enter script name: ")

	if script == "" {
		fmt.Printf("No script specified\n")
		return
	}

	if _, ok := scriptFuncs[script]; !ok {
		fmt.Printf("Unknown script: %s\n", script)
		return
	}

	scriptFuncs[script]()
}
