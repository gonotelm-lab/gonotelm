package util

import (
	"reflect"
	"slices"
	"strings"
)

func getFields(v any, keep func(v string) bool) (string, []string) {
	var tags []string
	val := reflect.TypeOf(v)

	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	// 只处理结构体类型
	if val.Kind() != reflect.Struct {
		return "", nil
	}

	for i := range val.NumField() {
		field := val.Field(i)
		if field.PkgPath != "" {
			continue
		}

		// 获取 clickhouse tag
		tag := field.Tag.Get("ch")
		if tag == "" {
			continue
		}

		// 可以跳过某些字段
		if keep(tag) {
			tags = append(tags, "`"+tag+"`")
		}
	}

	return strings.Join(tags, ","), tags
}

func GetFields(v any, skip ...string) string {
	s, _ := getFields(v, func(tag string) bool {
		return !slices.Contains(skip, tag)
	})

	return s
}
