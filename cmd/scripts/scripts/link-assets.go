package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

func LinkAssetsScript() {
	tracksFile := GetParam(2, "Enter file with bmm tracks: ")

	jsonFile, err := os.ReadFile(tracksFile)
	if err != nil {
		panic(err)
	}

	tracks := []*BmmTrack{}
	err = json.Unmarshal(jsonFile, &tracks)
	if err != nil {
		panic(err)
	}

	vxItemsFile := GetParam(3, "Enter file with vxitems: ")

	jsonFile, err = os.ReadFile(vxItemsFile)
	if err != nil {
		panic(err)
	}

	vxItems := []*VxItem{}
	err = json.Unmarshal(jsonFile, &vxItems)
	if err != nil {
		panic(err)
	}

	for _, track := range tracks {
		var vxItem *VxItem
		for _, item := range vxItems {
			if strings.Contains(item.Title, fmt.Sprint(track.ID)) {
				vxItem = item
				break
			}
		}
		if vxItem == nil {
			spew.Dump(track)
			panic("vxitem not found")
		}

		err := linkBmmEpisodeToNewestAsset(vxItem, track)
		if err != nil {
			panic(err)
		}
		return

	}

}

func linkBmmEpisodeToNewestAsset(item *VxItem, bmmTrack *BmmTrack) error {
	// Get episode for item
	db := requireSql()

	type Episode struct {
		ID int `json:"id"`
	}
	row := db.QueryRow("SELECT id FROM episodes WHERE label like $1 ORDER BY id desc LIMIT 1", fmt.Sprintf("BMM-%d%%", bmmTrack.ID))
	if row.Err() != nil {
		return row.Err()
	}

	var episodeId int
	err := row.Scan(&episodeId)
	if err != nil {
		return err
	}

	spew.Dump(episodeId)

	// Get asset for item
	var assetId int
	row = db.QueryRow("SELECT id FROM assets WHERE mediabanken_id = $1 ORDER BY id desc", item.VXID)
	if row.Err() != nil {
		return row.Err()
	}

	err = row.Scan(&assetId)
	if err != nil {
		return err
	}

	spew.Dump(assetId)

	// Link episode to asset
	db.Exec("UPDATE episodes SET asset_id = $1 WHERE id = $2", assetId, episodeId)

	return nil
}
