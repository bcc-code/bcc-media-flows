package notion

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

const exampleOutput = `[
  {
    "archived": false,
    "cover": null,
    "created_by": {
      "id": "dbefed57-cbd6-45a3-998a-feb07c3b8a2e",
      "object": "user"
    },
    "created_time": "2025-07-02T09:30:00.000Z",
    "icon": null,
    "id": "224563c0-cccf-80c9-a190-ecfc7a51f6f1",
    "in_trash": false,
    "last_edited_by": {
      "id": "dbefed57-cbd6-45a3-998a-feb07c3b8a2e",
      "object": "user"
    },
    "last_edited_time": "2025-07-02T09:30:00.000Z",
    "object": "page",
    "parent": {
      "database_id": "224563c0-cccf-8009-a7dd-c2d4638653a1",
      "type": "database_id"
    },
    "properties": {
      "Blah": {
        "id": "O%7D%3AM",
        "number": -3323.323,
        "type": "number"
      },
      "Checkbox": {
        "checkbox": false,
        "id": "AkFo",
        "type": "checkbox"
      },
      "Multi-select": {
        "id": "PxU%7D",
        "multi_select": [
          {
            "color": "yellow",
            "id": "3bef06ec-a033-49b1-a25c-477ddf6fe893",
            "name": "S"
          }
        ],
        "type": "multi_select"
      },
      "String field": {
        "id": "title",
        "title": [
          {
            "annotations": {
              "bold": false,
              "code": false,
              "color": "default",
              "italic": false,
              "strikethrough": false,
              "underline": false
            },
            "href": null,
            "plain_text": "BB",
            "text": {
              "content": "BB",
              "link": null
            },
            "type": "text"
          }
        ],
        "type": "title"
      },
      "Thing": {
        "id": "dIO%3E",
        "number": 777,
        "type": "number"
      }
    },
    "public_url": null,
    "url": "https://www.notion.so/BB-224563c0cccf80c9a190ecfc7a51f6f1"
  },
  {
    "archived": false,
    "cover": null,
    "created_by": {
      "id": "dbefed57-cbd6-45a3-998a-feb07c3b8a2e",
      "object": "user"
    },
    "created_time": "2025-07-02T08:43:00.000Z",
    "icon": null,
    "id": "224563c0-cccf-801a-9615-d78100241faf",
    "in_trash": false,
    "last_edited_by": {
      "id": "dbefed57-cbd6-45a3-998a-feb07c3b8a2e",
      "object": "user"
    },
    "last_edited_time": "2025-07-02T08:44:00.000Z",
    "object": "page",
    "parent": {
      "database_id": "224563c0-cccf-8009-a7dd-c2d4638653a1",
      "type": "database_id"
    },
    "properties": {
      "Blah": {
        "id": "O%7D%3AM",
        "number": 33.44,
        "type": "number"
      },
      "Checkbox": {
        "checkbox": true,
        "id": "AkFo",
        "type": "checkbox"
      },
      "Multi-select": {
        "id": "PxU%7D",
        "multi_select": [
          {
            "color": "red",
            "id": "5a2236e0-6174-4a5f-b1cb-afc40d1cd363",
            "name": "A"
          },
          {
            "color": "default",
            "id": "a5324b94-d163-4f2d-95f6-710aa67441ca",
            "name": "F"
          },
          {
            "color": "pink",
            "id": "91d9e629-7ba1-47bc-b818-71833639325b",
            "name": "G"
          }
        ],
        "type": "multi_select"
      },
      "String field": {
        "id": "title",
        "title": [
          {
            "annotations": {
              "bold": false,
              "code": false,
              "color": "default",
              "italic": false,
              "strikethrough": false,
              "underline": false
            },
            "href": null,
            "plain_text": "AA",
            "text": {
              "content": "AA",
              "link": null
            },
            "type": "text"
          }
        ],
        "type": "title"
      },
      "Thing": {
        "id": "dIO%3E",
        "number": 2,
        "type": "number"
      }
    },
    "public_url": null,
    "url": "https://www.notion.so/AA-224563c0cccf801a9615d78100241faf"
  }
]
`

type TestStruct struct {
	StringField string   `notion:"String field"`
	Thing       int      `notion:"Thing"`
	Blah        float64  `notion:"Blah"`
	Checkbox    bool     `notion:"Checkbox"`
	MultiSelect []string `notion:"Multi-select"`
}

func TestNotionToStruct(t *testing.T) {
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(exampleOutput), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	expected := []TestStruct{
		{
			StringField: "BB",
			Thing:       777,
			Blah:        -3323.323,
			Checkbox:    false,
			MultiSelect: []string{"S"},
		},
		{
			StringField: "AA",
			Thing:       2,
			Blah:        33.44,
			Checkbox:    true,
			MultiSelect: []string{"A", "F", "G"},
		},
	}

	actual, err := NotionToStruct[TestStruct](rows)
	assert.NoError(t, err)
	assert.Equal(t, len(expected), len(actual))

	for i, exp := range expected {
		act := actual[i]
		assert.Equal(t, exp.StringField, act.StringField)
		assert.Equal(t, exp.Thing, act.Thing)
		assert.InDelta(t, exp.Blah, act.Blah, 0.00001)
		assert.Equal(t, exp.Checkbox, act.Checkbox)
		assert.Equal(t, exp.MultiSelect, act.MultiSelect)
	}
}
