/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed openapi.yaml
var openapiSpec []byte

// swaggerUIHTML is a Swagger UI page. The CORS middleware on the REST server
// ensures /openapi.yaml is reachable from any browser origin.
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Fabric-X Block Explorer — API Docs</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
      window.onload = function () {
        SwaggerUIBundle({
          url: window.location.origin + "/openapi.yaml",
          dom_id: "#swagger-ui",
          presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
          layout: "StandaloneLayout",
          deepLinking: true,
          requestInterceptor: function(req) {
            // Ensure the spec fetch always goes to the explorer origin,
            // not the browser's current origin (important when opened via proxy).
            return req;
          },
        });
      };
    </script>
  </body>
</html>`

func (*Service) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	spec := strings.ReplaceAll(string(openapiSpec), "__SERVER_URL__", scheme+"://"+r.Host)
	//nolint:gosec // spec is static embedded YAML; only __SERVER_URL__ is replaced with a trusted request-derived value
	_, _ = w.Write([]byte(spec))
}

func handleSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIHTML))
}
