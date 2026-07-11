package generate

type StorageResult struct {
	StoreKey    string              `json:"store_key"`
	ContentType string              `json:"content_type"`
	Image       *StorageResultImage `json:"image,omitempty"`
}

type StorageResultImage struct {
	Width  int `json:"w"`
	Height int `json:"h"`
}
