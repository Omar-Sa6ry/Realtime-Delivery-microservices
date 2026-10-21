package graphql

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Schema struct {
	sdl string
}

func (s *Schema) SetSDLSdl(sdl string) {
	s.sdl = sdl
}

func (s *Schema) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Handle _service introspection query
		if strings.Contains(req.Query, "_service") {
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"_service": map[string]interface{}{
						"sdl": s.sdl,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Simple query handling - return typename
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"__typename": "Query",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}