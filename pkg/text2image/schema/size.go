package schema

import "strings"

// 将size中的乘号统一成 x
func ConvSizeX(size string) string {
	return strings.ReplaceAll(size, "*", "x")
}

// 将size中的乘号统一成 *
func ConvSizeMul(size string) string {
	return strings.ReplaceAll(size, "x", "*")
}
