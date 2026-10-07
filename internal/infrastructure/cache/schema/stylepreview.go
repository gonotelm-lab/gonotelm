package schema

type StylePreview struct {
	StoreKey         string `json:"store_key"         msgpack:"store_key"`
	Identifier       string `json:"identifier"        msgpack:"identifier"`
	Md5sum           []byte `json:"md5sum"            msgpack:"md5sum"`
	OriginalFilename string `json:"original_filename" msgpack:"original_filename"`
	CreateTime       int64  `json:"create_time"       msgpack:"create_time"`
	UpdateTime       int64  `json:"update_time"       msgpack:"update_time"`
}
