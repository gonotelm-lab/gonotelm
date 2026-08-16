package workspace

import (
	"embed"
	_ "embed"
	"io/fs"
)

var (
	//go:embed vendor/**
	vendor embed.FS

	vendorMaps map[string]fs.File = make(map[string]fs.File)
)

func init() {
	var err error
	standaloneCjsFile, err := vendor.Open("vendor/standalone.cjs")
	if err != nil {
		panic(err)
	}

	vendorMaps["standalone.cjs"] = standaloneCjsFile
}

func Vendors() map[string]fs.File {
	return vendorMaps
}
