// docs.go — Interactive API documentation UI.
//
// Serves the Scalar API-reference shell, which renders the live spec from
// /openapi.json in the browser. This mirrors Huma's built-in Scalar renderer
// (huma.DocsRendererScalar); the shell loads from the Scalar CDN, so the route
// carries its own CSP that relaxes the stricter global one for this page only.
package openapi

import "net/http"

// docsHTML is the Scalar API-reference shell pointing at the live JSON spec.
const docsHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="referrer" content="no-referrer">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>ishos API Reference</title>
  </head>
  <body>
    <script id="api-reference" data-url="/openapi.json"></script>
    <script src="https://unpkg.com/@scalar/api-reference@1.44.20/dist/browser/standalone.js" crossorigin integrity="sha384-tMz7GAo6dMy55x9tLFtH+sHtogji6Scmb+feBR31TAHmvSPRUTboK9H3M5NFaP4R"></script>
  </body>
</html>`

// docsCSP permits the pinned Scalar CDN script (and its eval) plus inline
// styles, keeping every other source blocked. Overrides the global CSP on
// this route only.
const docsCSP = "default-src 'none'; " +
	"base-uri 'none'; " +
	"connect-src 'self'; " +
	"form-action 'none'; " +
	"frame-ancestors 'none'; " +
	"sandbox allow-same-origin allow-scripts allow-popups allow-popups-to-escape-sandbox; " +
	"script-src 'unsafe-eval' https://unpkg.com/@scalar/api-reference@1.44.20/dist/browser/standalone.js; " +
	"style-src 'unsafe-inline'"

// DocsHandler serves the interactive Scalar documentation UI for the API.
func DocsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", docsCSP)
	_, _ = w.Write([]byte(docsHTML))
}
