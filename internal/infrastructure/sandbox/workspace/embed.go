package workspace

import (
	"embed"
	"io/fs"
)

//go:embed vendor/**
var vendor embed.FS

// Vendors 每次返回新打开的 fs.File。
// embed.FS 的 File 读完后 offset 停在 EOF，若复用同一句柄则后续上传会得到 0 字节文件。
func Vendors() map[string]fs.File {
	standalone, err := vendor.Open("vendor/standalone.cjs")
	if err != nil {
		panic(err)
	}
	return map[string]fs.File{
		"standalone.cjs": standalone,
	}
}
