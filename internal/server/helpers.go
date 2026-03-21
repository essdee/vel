package server

import (
	"encoding/json"
	"io"
	"net/http"
)

const maxRequestBodySize = 1 << 20 // 1MB

// decodeJSONBody decodes a JSON request body with a size limit.
func decodeJSONBody(r *http.Request, v interface{}) error {
	return json.NewDecoder(io.LimitReader(r.Body, maxRequestBodySize)).Decode(v)
}
