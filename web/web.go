// Package web embeds the static files of the web interface.
package web

import "embed"

// FS holds index.html, style.css and app.js.
//
//go:embed index.html style.css app.js
var FS embed.FS
