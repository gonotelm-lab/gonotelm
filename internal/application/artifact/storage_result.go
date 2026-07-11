package artifact

import "github.com/bytedance/sonic"

type storageResult struct {
	StoreKey    string `json:"StoreKey"`
	ContentType string `json:"ContentType"`
}

func extractStoreKey(result []byte) string {
	var sr storageResult
	if err := sonic.Unmarshal(result, &sr); err != nil {
		return ""
	}
	return sr.StoreKey
}
