package web

import "embed"

//go:embed templates/*.tmpl
var Templates embed.FS

//go:embed static/*
var Static embed.FS
