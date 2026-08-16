package opensandbox

import "net/http"

type Config struct {
	// opensandbox-server endpoint with schema, for example: http://127.0.0.1:8080
	Endpoint string

	ApiKey string

	HttpClient *http.Client

	Image string
}
