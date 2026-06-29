package util

import (
	"regexp"
)

var maybeMarkdownHeadingReg = regexp.MustCompile(`(?m)^#{1,6}\s+`)

const checkLen = 512

func MaybeHasMarkdownHeading(text string) bool {
	return maybeMarkdownHeadingReg.MatchString(text[0:min(len(text), checkLen)])
}

func MaybeHasMarkdownHeadingBytes(b []byte) bool {
	return maybeMarkdownHeadingReg.Match(b[0:min(len(b), checkLen)])
}
