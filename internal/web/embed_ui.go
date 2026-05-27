//go:build embedui

package web

import (
	"embed"
	"io/fs"
)

// 仅在 `go build -tags embedui` 时启用 — 此时 internal/web/static/ 必须
// 已经由 `make ui-build` 填好 (index.html + assets/...)。默认构建不引用
// 这一组文件, 所以仓库里不需要保留任何占位文件 / .gitkeep。
//
//go:embed all:static
var embedded embed.FS

func init() {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err)
	}
	uiFS = sub
}
