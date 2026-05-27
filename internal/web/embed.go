package web

import (
"io/fs"
"net/http"
"path"
"strings"

"github.com/gin-gonic/gin"
)

// uiFS is wired up by embed_ui.go when compiled with -tags embedui.
// Default builds leave it nil and a placeholder page is served.
var uiFS fs.FS

const uiNotBuiltHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>ssh-tunnel-service - UI not built</title>
<style>body{font-family:system-ui,sans-serif;padding:2rem;max-width:640px;margin:auto;color:#0f172a}code{background:#f1f5f9;padding:0.15rem 0.4rem;border-radius:4px}</style>
</head><body><h1>SSH Tunnel Service — UI not built</h1>
<p>The binary does not have the SPA embedded (dev mode or built without <code>-tags embedui</code>).</p>
<p>Development: run <code>make ui-dev</code> in another terminal and open the vite dev server (default port 5173).</p>
<p>Release: run <code>make build</code> to compile the frontend and embed it in the Go binary.</p>
<p>The REST API is fully operational at <code>/api/*</code>.</p></body></html>`

// Mount registers the SPA shell and history-mode fallback routes.
func Mount(router *gin.Engine) {
ui := handler()
router.GET("/", gin.WrapH(ui))
router.NoRoute(func(c *gin.Context) {
if strings.HasPrefix(c.Request.URL.Path, "/api/") {
c.Status(http.StatusNotFound)
return
}
ui.ServeHTTP(c.Writer, c.Request)
})
}

func handler() http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
if uiFS == nil {
servePlaceholder(w, r)
return
}
serveEmbedded(w, r, uiFS)
})
}

func servePlaceholder(w http.ResponseWriter, r *http.Request) {
clean := path.Clean("/" + r.URL.Path)
if clean != "/" && path.Ext(clean) != "" {
http.NotFound(w, r)
return
}
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.WriteHeader(http.StatusOK)
_, _ = w.Write([]byte(uiNotBuiltHTML))
}

func serveEmbedded(w http.ResponseWriter, r *http.Request, sub fs.FS) {
files := http.FileServer(http.FS(sub))
name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
if name == "" || name == "." {
name = "index.html"
}
if name != "index.html" {
if info, err := fs.Stat(sub, name); err == nil && !info.IsDir() {
files.ServeHTTP(w, r)
return
}
if path.Ext(name) != "" {
http.NotFound(w, r)
return
}
}
body, err := fs.ReadFile(sub, "index.html")
if err != nil {
body = []byte(uiNotBuiltHTML)
}
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.WriteHeader(http.StatusOK)
_, _ = w.Write(body)
}
