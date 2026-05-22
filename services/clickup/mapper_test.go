package clickup

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixture from the live "Shorts Export" list (task 86c9t42wu).
const fixtureTaskJSON = `{
  "id": "86c9t42wu",
  "name": "HISM_20190414_1900_MAS_NOR",
  "custom_fields": [
    {
      "id": "a53c5f4d-39ac-4fd0-bbf4-715056e6d495",
      "name": "Asset status",
      "type": "drop_down",
      "type_config": {
        "options": [
          {"id": "4d9be79c-54af-43a1-8e2a-7721aba00443", "name": "In progress", "orderindex": 0},
          {"id": "b77e8099-a8da-4f51-9e14-b328a023d54a", "name": "Done",        "orderindex": 1}
        ]
      },
      "value": 0
    },
    {
      "id": "e634fb82-fcdc-42c8-8179-d004a5df03ed",
      "name": "Editorial status",
      "type": "drop_down",
      "type_config": {
        "options": [
          {"id": "a4f649a4-9cb5-407e-8fbf-762d96e3ec16", "name": "Not started",          "orderindex": 0},
          {"id": "e206635a-1545-4abb-8b83-7f1ad3e33573", "name": "Not approved",         "orderindex": 1},
          {"id": "2da57260-64c5-4721-964a-c23c4d18fa8a", "name": "On hold",              "orderindex": 2},
          {"id": "1728daba-e4ec-4f88-942c-176596bc828a", "name": "Approved",             "orderindex": 3},
          {"id": "13becc99-ccbf-436a-a892-58709acbe6c7", "name": "Ready in Mediabanken", "orderindex": 4}
        ]
      },
      "value": 4
    },
    {
      "id": "ecdd2ba7-1007-4568-ae27-a5a5c51492d8",
      "name": "Episode ID",
      "type": "short_text",
      "type_config": {},
      "value": "2898"
    },
    {
      "id": "9a21b3ae-9503-4653-a6de-2c82b4f7ed11",
      "name": "IN",
      "type": "short_text",
      "type_config": {},
      "value": "32:37"
    },
    {
      "id": "32402e82-acfe-4af9-a19c-4dcb4cf975c8",
      "name": "OUT",
      "type": "short_text",
      "type_config": {},
      "value": "33:37"
    }
  ]
}`

func parseFixture(t *testing.T) Task {
	t.Helper()
	var task Task
	require.NoError(t, json.Unmarshal([]byte(fixtureTaskJSON), &task))
	return task
}

func TestTaskField(t *testing.T) {
	task := parseFixture(t)
	assert.NotNil(t, task.Field(FieldEpisodeID))
	assert.Nil(t, task.Field("does-not-exist"))
}

func TestShortText(t *testing.T) {
	task := parseFixture(t)
	assert.Equal(t, "2898", task.Field(FieldEpisodeID).ShortText())
	assert.Equal(t, "32:37", task.Field(FieldInID).ShortText())
	assert.Equal(t, "33:37", task.Field(FieldOutID).ShortText())

	var nilCF *CustomField
	assert.Equal(t, "", nilCF.ShortText())
}

func TestDropDownName(t *testing.T) {
	task := parseFixture(t)
	assert.Equal(t, "In progress", task.Field(FieldAssetStatusID).DropDownName())
	assert.Equal(t, EditorialReadyInMediabanken, task.Field(FieldEditorialStatusID).DropDownName())

	var nilCF *CustomField
	assert.Equal(t, "", nilCF.DropDownName())
}

func TestDropDownNameNullValue(t *testing.T) {
	cf := &CustomField{
		Type:  "drop_down",
		Value: json.RawMessage("null"),
		TypeConfig: TypeConfig{
			Options: []DropDownOption{{ID: "x", Name: "x", OrderIndex: 0}},
		},
	}
	assert.Equal(t, "", cf.DropDownName())
}
