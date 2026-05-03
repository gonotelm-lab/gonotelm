package impl

import "time"

type Config struct {
	Type Type `toml:"type"`

	Milvus *MilvusConfig `toml:"milvus"`
}

type MilvusConfig struct {
	Addr        string        `toml:"addr"`
	Username    string        `toml:"username"`
	Password    string        `toml:"password"`
	DBName      string        `toml:"dbName"`
	DialTimeout time.Duration `toml:"dialTimeout"`
}
