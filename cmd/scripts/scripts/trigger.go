package scripts

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-resty/resty/v2"
)

func TriggerItemsScript() {
	file := GetParam(2, "Enter file: ")
	destinations := GetParam(3, "Enter destinations: ")

	jsonFile, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	items := []*VxItem{}
	err = json.Unmarshal(jsonFile, &items)
	if err != nil {
		panic(err)
	}

	err = triggerExportForVxItems(items, destinations)
	if err != nil {
		panic(err)
	}
}

func triggerExportForVxItems(items []*VxItem, destinations string) error {
	restyClient := resty.New()

	watermarkPath := "/mnt/isilon/system/overlay/BTV_LOGO_WATERMARK_BUG_GFX_1080.png"

	for index, item := range items {
		result, err := restyClient.GetClient().Get(fmt.Sprintf("https://temporal-trigger.lan.bcc.media/trigger/ExportAssetVX?destinations=%s&watermarkPath=%s&vxID=%s", destinations, watermarkPath, item.VXID))
		if err != nil {
			spew.Dump(result)
			fmt.Printf("Stopped at index %d\n", index)
			return err
		}
		fmt.Printf("Triggered export for %s. Result: %v\n", item.VXID, result)
	}

	return nil
}
