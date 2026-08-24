package util

import (
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
)

type RenamedRunInfo struct {
	Name      string               `json:"name"`
	Type      string               `json:"type"`
	Component components.Component `json:"component"`
}

func RenameRunInfo(info *callbacks.RunInfo) *RenamedRunInfo {
	if info == nil {
		return nil
	}

	return &RenamedRunInfo{
		Name:      info.Name,
		Type:      info.Type,
		Component: info.Component,
	}
}
