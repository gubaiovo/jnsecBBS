// Package frontend 内嵌 Bun/Vite 构建的 React SPA 产物。
// 构建顺序：先在 frontend/ 执行 bun run build（输出到 internal/frontend/dist），再编译 Go。
package frontend

import "embed"

//go:embed all:dist
var Dist embed.FS
