package clickup

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// assetStatusOptions / editorialStatusOptions mirror the option definitions
// returned by the public view-load call.
var assetStatusOptions = []DropDownOption{
	{ID: "4d9be79c-54af-43a1-8e2a-7721aba00443", Name: "In progress"},
	{ID: "b77e8099-a8da-4f51-9e14-b328a023d54a", Name: "Done"},
}

var editorialStatusOptions = []DropDownOption{
	{ID: "a4f649a4-9cb5-407e-8fbf-762d96e3ec16", Name: "Not started"},
	{ID: "e206635a-1545-4abb-8b83-7f1ad3e33573", Name: "Not approved"},
	{ID: "2da57260-64c5-4721-964a-c23c4d18fa8a", Name: "On hold"},
	{ID: "1728daba-e4ec-4f88-942c-176596bc828a", Name: "Approved"},
	{ID: "13becc99-ccbf-436a-a892-58709acbe6c7", Name: "Ready in Mediabanken"},
}

// fixtureTask mirrors a task assembled from the two public-view calls (live task
// 86c9zwrv5): drop_down values are option UUIDs, short_text values are strings.
func fixtureTask() Task {
	return Task{
		ID:   "86c9zwrv5",
		Name: "PI26_20260523_1900_PGM_MU1",
		CustomFields: []CustomField{
			{
				ID:         FieldAssetStatusID,
				Name:       "Asset status",
				Type:       "drop_down",
				TypeConfig: TypeConfig{Options: assetStatusOptions},
				Value:      json.RawMessage(`"4d9be79c-54af-43a1-8e2a-7721aba00443"`),
			},
			{
				ID:         FieldEditorialStatusID,
				Name:       "Editorial status",
				Type:       "drop_down",
				TypeConfig: TypeConfig{Options: editorialStatusOptions},
				Value:      json.RawMessage(`"13becc99-ccbf-436a-a892-58709acbe6c7"`),
			},
			{ID: FieldEpisodeID, Name: "Episode ID", Type: "short_text", Value: json.RawMessage(`"3046"`)},
			{ID: FieldInID, Name: "IN", Type: "short_text", Value: json.RawMessage(`"8:55"`)},
			{ID: FieldOutID, Name: "OUT", Type: "short_text", Value: json.RawMessage(`"9:25"`)},
		},
	}
}

func TestTaskField(t *testing.T) {
	task := fixtureTask()
	assert.NotNil(t, task.Field(FieldEpisodeID))
	assert.Nil(t, task.Field("does-not-exist"))
}

func TestShortText(t *testing.T) {
	task := fixtureTask()
	assert.Equal(t, "3046", task.Field(FieldEpisodeID).ShortText())
	assert.Equal(t, "8:55", task.Field(FieldInID).ShortText())
	assert.Equal(t, "9:25", task.Field(FieldOutID).ShortText())

	var nilCF *CustomField
	assert.Equal(t, "", nilCF.ShortText())
}

func TestDropDownName(t *testing.T) {
	task := fixtureTask()
	assert.Equal(t, "In progress", task.Field(FieldAssetStatusID).DropDownName())
	assert.Equal(t, EditorialReadyInMediabanken, task.Field(FieldEditorialStatusID).DropDownName())

	var nilCF *CustomField
	assert.Equal(t, "", nilCF.DropDownName())
}

func TestDropDownNameNullValue(t *testing.T) {
	cf := &CustomField{
		Type:       "drop_down",
		Value:      json.RawMessage("null"),
		TypeConfig: TypeConfig{Options: []DropDownOption{{ID: "x", Name: "x"}}},
	}
	assert.Equal(t, "", cf.DropDownName())
}

// TestDropDownNameUnknownOption ensures an unmapped option UUID resolves to "".
func TestDropDownNameUnknownOption(t *testing.T) {
	cf := &CustomField{
		Type:       "drop_down",
		Value:      json.RawMessage(`"not-a-known-option"`),
		TypeConfig: TypeConfig{Options: assetStatusOptions},
	}
	assert.Equal(t, "", cf.DropDownName())
}
