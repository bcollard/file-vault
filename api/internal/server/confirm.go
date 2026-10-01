package server

import (
	"fmt"
	"html/template"
	"net/http"
)

// confirmTmpl is a deliberately minimal HTML page rendered by GET /d/{token}.
// Only the auto-escaped filename is templated — no other user-controlled
// content, no JavaScript, no external resources. The single form button
// POSTs to /d/{token}/confirm to perform the burn.
var confirmTmpl = template.Must(template.New("confirm").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="robots" content="noindex, nofollow">
<title>Download</title>
<style>
:root { color-scheme: light dark; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; max-width: 460px; margin: 80px auto; padding: 24px; line-height: 1.5; }
h1 { font-size: 18px; margin: 0 0 16px; }
.file { background: #f5f5f7; padding: 16px; border-radius: 8px; margin: 16px 0; }
.file-name { font-weight: 600; word-break: break-all; }
.file-meta { color: #6e6e73; font-size: 13px; margin-top: 4px; }
button { background: #0a84ff; color: #fff; border: 0; padding: 12px 20px; border-radius: 8px; font-size: 15px; font-weight: 600; cursor: pointer; width: 100%; }
.warn { color: #c5221f; font-size: 13px; margin-top: 16px; text-align: center; }
@media (prefers-color-scheme: dark) {
  body { background: #1c1c1e; color: #f5f5f7; }
  .file { background: #2c2c2e; }
  .file-meta { color: #98989d; }
}
</style>
</head>
<body>
<h1>One-time download</h1>
<div class="file">
  <div class="file-name">{{.Filename}}</div>
  <div class="file-meta">{{.SizeReadable}}</div>
</div>
<form method="POST" action="{{.ConfirmPath}}">
  <button type="submit">Download once</button>
</form>
<p class="warn">This link works once. After download it becomes invalid.</p>
</body>
</html>`))

type confirmData struct {
	Filename     string
	SizeReadable string
	ConfirmPath  string
}

func renderConfirm(w http.ResponseWriter, token, filename string, size int64) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	_ = confirmTmpl.Execute(w, confirmData{
		Filename:     filename,
		SizeReadable: humanSize(size),
		ConfirmPath:  "/d/" + token + "/confirm",
	})
}

func humanSize(b int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
		GiB = 1024 * MiB
	)
	switch {
	case b >= GiB:
		return fmt.Sprintf("%.2f GiB", float64(b)/float64(GiB))
	case b >= MiB:
		return fmt.Sprintf("%.2f MiB", float64(b)/float64(MiB))
	case b >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
