package web

import "embed"

// Files содержит только файлы интерфейса, необходимые работающему приложению.
//
//go:embed index.html assets/*
var Files embed.FS
