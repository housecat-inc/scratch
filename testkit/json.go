package testkit

import (
	"encoding/json"
	"reflect"
	"sort"
)

func JSON(got any) []string {
	items := flattenJSONItems(got)
	out := make([]string, 0, len(items))
	for _, item := range items {
		normalized := normalizeJSON(item)
		bytes, err := json.Marshal(normalized)
		if err != nil {
			panic(err)
		}
		out = append(out, string(bytes))
	}
	sort.Strings(out)
	return out
}

func flattenJSONItems(got any) []any {
	value := reflect.ValueOf(got)
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return []any{got}
	}

	items := make([]any, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		items = append(items, value.Index(i).Interface())
	}
	return items
}

func normalizeJSON(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		panic(err)
	}
	return normalizeJSONValue(normalized)
}

func normalizeJSONValue(value any) any {
	switch value := value.(type) {
	case []any:
		for i := range value {
			value[i] = normalizeJSONValue(value[i])
		}
		sort.Slice(value, func(i, j int) bool {
			left, _ := json.Marshal(value[i])
			right, _ := json.Marshal(value[j])
			return string(left) < string(right)
		})
		return value
	case map[string]any:
		for key := range value {
			value[key] = normalizeJSONValue(value[key])
		}
		return value
	default:
		return value
	}
}
