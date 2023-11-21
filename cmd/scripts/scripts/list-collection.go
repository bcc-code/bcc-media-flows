package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
)

func GetVxItemsFromCollectionScript(vsapiClient *vsapi.Client) {
	id := GetParam(2, "Enter collection id: ")

	val, err := getVxItemsFromCollection(vsapiClient, id)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d items\n", len(val))

	json, err := json.Marshal(val)
	_ = os.Mkdir("output", 0755)
	os.WriteFile(path.Join("output", string(id)+".json"), []byte(json), 0644)

	fmt.Printf("Wrote %d items to %s\n", len(val), path.Join("output", string(id)+".json"))
}

type VxItem struct {
	VXID  string
	Title string
}

func getVxItemsFromCollection(vsapiClient *vsapi.Client, collectionId string) ([]*VxItem, error) {
	val, err := vsapiClient.GetItemsInCollection(collectionId, 1000)
	if err != nil {
		return nil, err
	}

	items := []*VxItem{}
	for _, item := range val.Items {
		vxItem := &VxItem{
			VXID:  item.ID,
			Title: item.Get(vscommon.FieldTitle, ""),
		}
		items = append(items, vxItem)
	}

	return items, nil
}
