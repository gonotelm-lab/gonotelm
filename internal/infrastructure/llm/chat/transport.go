package chat

import (
	"net/http"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/requestid"
)

// opencodeSessionHeader is the session identifier header required by the
// OpenCode Go provider. Requests missing it are rejected with a
// 400 MissingSessionID error.
const opencodeSessionHeader = "x-opencode-session"

// opencodeSessionTransport injects the OpenCode Go session header. The session
// id is derived from the scene group id, which groups all LLM calls belonging
// to a single conversation flow, so that requests stay routable and cacheable.
type opencodeSessionTransport struct {
	next http.RoundTripper
}

func (t *opencodeSessionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get(opencodeSessionHeader) == "" {
		sessionID := pkgcontext.GetSceneGroupId(req.Context())
		if sessionID == "" {
			sessionID = pkgcontext.GetReqId(req.Context()).String()
		}
		if sessionID == "" {
			sessionID = pkgcontext.GetUserId(req.Context()).String()
		}
		if sessionID == "" {
			sessionID = requestid.Gen().String()
		}

		req = req.Clone(req.Context())
		req.Header.Set(opencodeSessionHeader, sessionID)
	}

	return t.next.RoundTrip(req)
}
