package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"
)

// swaggerHTML loads Swagger UI from a CDN. The request interceptor adds
// Content-Type: application/json to bodiless POSTs (block, publish, …) so
// they pass the RequireJSON middleware, and sends cookies.
const swaggerHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>StreakIt API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js" crossorigin></script>
<script>
window.ui = SwaggerUIBundle({
  url: "doc.json",
  dom_id: "#swagger-ui",
  withCredentials: true,
  persistAuthorization: true,
  requestInterceptor: function (req) {
    var m = (req.method || "GET").toUpperCase();
    if (m !== "GET" && m !== "HEAD" && !req.headers["Content-Type"]) {
      req.headers["Content-Type"] = "application/json";
    }
    return req;
  }
});
</script>
</body>
</html>`

// SwaggerUI serves the UI page. Mounted when SWAGGER_ENABLED (default true).
func SwaggerUI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
}

// SwaggerDoc serves the generated OpenAPI document.
func SwaggerDoc(c *gin.Context) {
	doc, err := swag.ReadDoc()
	if err != nil {
		c.String(http.StatusInternalServerError, "swagger doc not generated: run make swagger")
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(doc))
}
