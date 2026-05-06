package conf

type LogicConfig struct {
	Chat ChatLogicConfig `toml:"chat"`
}

type ChatLogicConfig struct {
	SourceDocsRecallCount int `toml:"sourceDocsRecallCount"`
}
