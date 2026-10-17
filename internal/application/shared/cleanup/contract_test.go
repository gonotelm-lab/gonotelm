package cleanup

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPayloadRoundTrip(t *testing.T) {
	Convey("Payload marshal/unmarshal round-trip", t, func() {
		notebookId := valobj.NewId()
		sourceId := valobj.NewId()
		fileKey, err := valobj.NewStoreKey("gonotelm-private", "file/"+notebookId.String()+"/"+sourceId.String()+".pdf", false)
		So(err, ShouldBeNil)

		p := &Payload{
			Kind:       KindSource,
			NotebookId: notebookId,
			SourceId:   sourceId,
			Keys:       []string{fileKey.Encode()},
		}

		data, err := p.Marshal()
		So(err, ShouldBeNil)

		got, err := Unmarshal(data)
		So(err, ShouldBeNil)
		So(got.Kind, ShouldEqual, KindSource)
		So(got.NotebookId, ShouldEqual, notebookId)
		So(got.SourceId, ShouldEqual, sourceId)
		So(got.Keys, ShouldHaveLength, 1)
		So(got.Keys[0], ShouldEqual, fileKey.Encode())

		decoded, err := valobj.DecodeStoreKey(got.Keys[0])
		So(err, ShouldBeNil)
		So(decoded, ShouldEqual, fileKey)
	})
}

func TestPayloadRoundTripNotebook(t *testing.T) {
	Convey("Notebook payload keeps every source", t, func() {
		notebookId := valobj.NewId()
		sourceId := valobj.NewId()

		p := &Payload{
			Kind:       KindNotebook,
			NotebookId: notebookId,
			Sources: []SourceRef{
				{SourceId: sourceId, Keys: []string{"a"}},
				{SourceId: valobj.NewId()},
			},
		}

		data, err := p.Marshal()
		So(err, ShouldBeNil)
		// notebook 载荷不该带顶层 source_id 的零值
		So(string(data), ShouldNotContainSubstring, "00000000000000000000000000000000")

		got, err := Unmarshal(data)
		So(err, ShouldBeNil)
		So(got.Kind, ShouldEqual, KindNotebook)
		So(got.Sources, ShouldHaveLength, 2)
		So(got.Sources[0].SourceId, ShouldEqual, sourceId)
		So(got.Sources[0].Keys, ShouldHaveLength, 1)
		So(got.Sources[1].Keys, ShouldBeEmpty)
		So(got.Artifacts, ShouldBeEmpty)
	})
}

func TestPayloadRoundTripNotebookArtifacts(t *testing.T) {
	Convey("Notebook payload keeps every artifact key", t, func() {
		notebookId, artifactId := valobj.NewId(), valobj.NewId()
		key, err := valobj.NewStoreKey("gonotelm-private", "artifact/"+notebookId.String()+"/"+artifactId.String()+".pptx", false)
		So(err, ShouldBeNil)

		p := &Payload{
			Kind:       KindNotebook,
			NotebookId: notebookId,
			Artifacts:  []ArtifactRef{{ArtifactId: artifactId, Keys: []string{key.Encode()}}},
		}

		data, err := p.Marshal()
		So(err, ShouldBeNil)

		got, err := Unmarshal(data)
		So(err, ShouldBeNil)
		So(got.Sources, ShouldBeEmpty)
		So(got.Artifacts, ShouldHaveLength, 1)
		So(got.Artifacts[0].ArtifactId, ShouldEqual, artifactId)
		So(got.Artifacts[0].Keys, ShouldResemble, []string{key.Encode()})
	})
}

func TestPayloadRoundTripArtifact(t *testing.T) {
	Convey("Artifact payload keeps its id and key", t, func() {
		notebookId, artifactId := valobj.NewId(), valobj.NewId()
		key, err := valobj.NewStoreKey("gonotelm-private", "artifact/nb/art.pptx", false)
		So(err, ShouldBeNil)

		p := &Payload{
			Kind:       KindArtifact,
			NotebookId: notebookId,
			ArtifactId: artifactId,
			Keys:       []string{key.Encode()},
		}

		data, err := p.Marshal()
		So(err, ShouldBeNil)

		got, err := Unmarshal(data)
		So(err, ShouldBeNil)
		So(got.Kind, ShouldEqual, KindArtifact)
		So(got.ArtifactId, ShouldEqual, artifactId)
		So(got.Keys, ShouldResemble, []string{key.Encode()})
	})
}

func TestUnmarshalRejectsInvalidPayload(t *testing.T) {
	Convey("Unmarshal rejects invalid payloads", t, func() {
		notebookId := valobj.NewId()
		sourceId := valobj.NewId()

		Convey("unknown kind", func() {
			data, err := (&Payload{Kind: "nope", NotebookId: notebookId}).Marshal()
			So(err, ShouldBeNil)

			_, err = Unmarshal(data)
			So(err, ShouldNotBeNil)
		})

		Convey("missing notebook id", func() {
			data, err := (&Payload{Kind: KindSource, SourceId: sourceId}).Marshal()
			So(err, ShouldBeNil)

			_, err = Unmarshal(data)
			So(err, ShouldNotBeNil)
		})

		Convey("source kind without source id", func() {
			data, err := (&Payload{Kind: KindSource, NotebookId: notebookId}).Marshal()
			So(err, ShouldBeNil)

			_, err = Unmarshal(data)
			So(err, ShouldNotBeNil)
		})

		Convey("artifact kind without artifact id", func() {
			data, err := (&Payload{Kind: KindArtifact, NotebookId: notebookId}).Marshal()
			So(err, ShouldBeNil)

			_, err = Unmarshal(data)
			So(err, ShouldNotBeNil)
		})

		Convey("not JSON", func() {
			_, err := Unmarshal([]byte("not-json"))
			So(err, ShouldNotBeNil)
		})
	})
}
