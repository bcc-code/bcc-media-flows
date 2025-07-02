package notion

import (
	"errors"
	"reflect"
)

type TestStruct struct {
	Name        string   `notion:"Name"`
	Thing       int      `notion:"Thing"`
	Blah        float64  `notion:"Blah"`
	Checkbox    bool     `notion:"Checkbox"`
	MultiSelect []string `notion:"Multi-select"`
}

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
		prop, ok := props[tag].(map[string]interface{})
		if !ok {
			continue
		}
		switch field.Type.Kind() {
		case reflect.String:
			if tag == "Name" {
				titles, ok := prop["title"].([]interface{})
				if ok && len(titles) > 0 {
					title, ok := titles[0].(map[string]interface{})
					if ok {
						plain, _ := title["plain_text"].(string)
						structVal.Field(j).SetString(plain)
					}
				}
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
			if tag == "Multi-select" {
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
	}
	return &out, nil
}

// NotionToStruct maps a slice of Notion rows to a slice of structs using `notion` tags
func NotionToStruct[T any](rows []map[string]interface{}) ([]T, error) {
	result := make([]T, 0, len(rows))
	for _, row := range rows {
		item, err := mapNotionRowToStruct[T](row)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, nil
}
