package convertdoc

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestExtractHTMLTitle(t *testing.T) {
	Convey("extractHTMLTitle", t, func() {
		cases := []struct {
			name string
			html string
			want string
		}{
			{
				name: "normal title in head",
				html: `<!DOCTYPE html><html><head><title>Hello World</title></head><body></body></html>`,
				want: "Hello World",
			},
			{
				name: "case insensitive tags",
				html: `<HTML><HEAD><TITLE>Mixed Case</TITLE></HEAD></HTML>`,
				want: "Mixed Case",
			},
			{
				name: "title with attributes and entities",
				html: `<head><title lang="zh">Foo &amp; Bar</title></head>`,
				want: "Foo & Bar",
			},
			{
				name: "trim surrounding whitespace and newlines",
				html: "<head>\n<title>\n  Spaced Title  \n</title>\n</head>",
				want: "Spaced Title",
			},
			{
				name: "head has attributes",
				html: `<head profile="http://example.com"><title>With Head Attr</title></head>`,
				want: "With Head Attr",
			},
			{
				name: "no title tag",
				html: `<html><head><meta charset="utf-8"></head><body>hi</body></html>`,
				want: "",
			},
			{
				name: "title outside head is ignored",
				html: `<html><body><title>Body Title</title></body></html>`,
				want: "",
			},
			{
				name: "empty body",
				html: "",
				want: "",
			},
		}

		for _, tc := range cases {
			Convey(tc.name, func() {
				So(extractHTMLTitle([]byte(tc.html)), ShouldEqual, tc.want)
			})
		}
	})
}
