package notion

import (
	"errors"
	"reflect"
)

func mapNotionRowToStruct[T any](data map[string]interface{}) (*T, error) {
	props, ok := data["properties"].(map[string]interface{})
	if !ok {
		return nil, errors.New("missing properties")
	}
	var out T
	structVal := reflect.ValueOf(&out).Elem()
	structType := structVal.Type()
	for j := 0; j < structType.NumField(); j++ {
		field := structType.Field(j)
		tag := field.Tag.Get("notion")
		if tag == "" {
			continue
		}

		// Special handling for rowId tag
		if tag == "rowId" && field.Type.Kind() == reflect.String {
			id, _ := data["id"].(string)
			structVal.Field(j).SetString(id)
			continue
		}

		prop, ok := props[tag].(map[string]interface{})
		if !ok {
			continue
		}

		propType, ok := prop["type"].(string)
		if !ok {
			continue
		}

		switch field.Type.Kind() {
		case reflect.String:
			// Use the Notion "title" property for string fields if present, otherwise fallback to rich_text or plain string
			titles, ok := prop["title"].([]interface{})
			if ok && len(titles) > 0 {
				title, ok := titles[0].(map[string]interface{})
				if ok {
					plain, _ := title["plain_text"].(string)
					structVal.Field(j).SetString(plain)
				}
				continue
			}
			richTexts, ok := prop["rich_text"].([]interface{})
			if ok && len(richTexts) > 0 {
				richText, ok := richTexts[0].(map[string]interface{})
				if ok {
					plain, _ := richText["plain_text"].(string)
					structVal.Field(j).SetString(plain)
				}
			}
			status, ok := prop["status"].(map[string]interface{})
			if ok {
				status, ok := status["name"].(string)
				if ok {
					structVal.Field(j).SetString(status)
				}
				continue
			}
			if propType == "select" {
				sel, ok := prop["select"].(map[string]interface{})
				if ok {
					structVal.Field(j).SetString(sel["name"].(string))
				}
			}
			// Fallback for string
			if s, ok := prop["name"].(string); ok {
				structVal.Field(j).SetString(s)
			}
		case reflect.Int, reflect.Int64:
			if num, ok := prop["number"].(float64); ok {
				structVal.Field(j).SetInt(int64(num))
			}
		case reflect.Float64:
			if num, ok := prop["number"].(float64); ok {
				structVal.Field(j).SetFloat(num)
			}
		case reflect.Bool:
			if b, ok := prop["checkbox"].(bool); ok {
				structVal.Field(j).SetBool(b)
			}
		case reflect.Slice:
			multi, ok := prop["multi_select"].([]interface{})
			if ok {
				var arr []string
				for _, m := range multi {
					mmap, ok := m.(map[string]interface{})
					if ok {
						name, _ := mmap["name"].(string)
						arr = append(arr, name)
					}
				}
				structVal.Field(j).Set(reflect.ValueOf(arr))
			}
		}
	}
	return &out, nil
}

// NotionToStruct maps a slice of Notion rows to a slice of structs using `notion` tags
func NotionToStruct[T any](rows []map[string]interface{}) ([]*T, error) {
	result := make([]*T, 0, len(rows))
	for _, row := range rows {
		item, err := mapNotionRowToStruct[T](row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}
