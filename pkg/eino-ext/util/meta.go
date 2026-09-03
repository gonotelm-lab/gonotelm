package util

import "maps"

func CopyMeta(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}

	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}
