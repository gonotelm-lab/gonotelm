package adapter

import (
	"bytes"
	"net/http"

	"github.com/cloudwego/hertz/pkg/protocol"
)

// GetCompatRequest converts a hertz request into a net/http request for compatibility.
// remoteAddr is the client network address (e.g. RequestContext.RemoteAddr().String());
// it populates the RemoteAddr field used for network.peer.* and client.address attributes.
func GetCompatRequest(req *protocol.Request, remoteAddr string) (*http.Request, error) {
	r, err := http.NewRequest(string(req.Method()), req.URI().String(), bytes.NewReader(req.Body()))
	if err != nil {
		return r, err
	}

	h := make(map[string][]string)
	req.Header.VisitAll(func(k, v []byte) {
		h[string(k)] = append(h[string(k)], string(v))
	})

	r.Header = h
	r.RemoteAddr = remoteAddr
	if r.Host == "" {
		r.Host = string(req.Header.Host())
	}
	return r, nil
}
