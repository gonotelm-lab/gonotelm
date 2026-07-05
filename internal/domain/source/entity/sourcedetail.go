package entity

type SourceDetail struct {
	Source *Source
	Access *SourceAccess
}

type SourceAccess struct {
	FileContentUrl   string
	ParsedContentUrl string
}
