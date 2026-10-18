package flow

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gonotelm-lab/flow/api/signature"
	. "github.com/smartystreets/goconvey/convey"
)

const (
	testSecret    = "sk-test-secret"
	testNamespace = "gonotelm"
	testPath      = "/inner-api/v1/flow/webhook/notebook-cleanup"
	testBody      = `{"kind":"notebook-cleanup"}`
)

func signedHeader(t *testing.T, timestamp time.Time) http.Header {
	t.Helper()

	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set(signature.HeaderScheduleID, "sched-1")
	header.Set(signature.HeaderFireID, "fire-1")
	header.Set(signature.HeaderScheduledTime, "1792201132661")
	header.Set(signature.HeaderTimestamp, strconv.FormatInt(timestamp.Unix(), 10))
	header.Set(signature.HeaderNamespace, testNamespace)

	sig, err := signature.Sign(testSecret, signature.Input{
		Method: http.MethodPost,
		URL:    &url.URL{Path: testPath},
		Header: header,
		Body:   []byte(testBody),
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	header.Set(signature.HeaderSignature, sig)

	return header
}

func verifyRequest(method, path string, header http.Header, body string) *http.Request {
	return &http.Request{
		Method:        method,
		URL:           &url.URL{Path: path},
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader([]byte(body))),
		ContentLength: int64(len(body)),
	}
}

func TestScheduleClientVerify(t *testing.T) {
	Convey("verifies a signature produced with the same derived key", t, func() {
		client, err := NewScheduleClient(Config{Namespace: testNamespace, APIKey: testSecret})
		So(err, ShouldBeNil)
		defer client.Close()

		header := signedHeader(t, time.Now())
		So(client.Verify(verifyRequest(http.MethodPost, testPath, header, testBody)), ShouldBeNil)
	})
}

func TestScheduleClientVerifyRejectsTamperedRequest(t *testing.T) {
	Convey("rejects tampered requests", t, func() {
		client, err := NewScheduleClient(Config{Namespace: testNamespace, APIKey: testSecret})
		So(err, ShouldBeNil)
		defer client.Close()

		header := signedHeader(t, time.Now())

		Convey("tampered body", func() {
			So(client.Verify(verifyRequest(http.MethodPost, testPath, header, testBody+"x")), ShouldNotBeNil)
		})

		Convey("tampered path", func() {
			So(client.Verify(verifyRequest(http.MethodPost, testPath+"x", header, testBody)), ShouldNotBeNil)
		})

		Convey("tampered method", func() {
			So(client.Verify(verifyRequest(http.MethodGet, testPath, header, testBody)), ShouldNotBeNil)
		})

		Convey("wrong secret", func() {
			wrong, err := NewScheduleClient(Config{Namespace: testNamespace, APIKey: "sk-other-secret"})
			So(err, ShouldBeNil)
			defer wrong.Close()

			So(wrong.Verify(verifyRequest(http.MethodPost, testPath, header, testBody)), ShouldNotBeNil)
		})

		Convey("stale timestamp", func() {
			old := signedHeader(t, time.Now().Add(-time.Hour))
			So(client.Verify(verifyRequest(http.MethodPost, testPath, old, testBody)), ShouldNotBeNil)
		})
	})
}
