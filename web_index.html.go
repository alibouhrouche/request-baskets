package main

import _ "embed"

var (
	//go:embed templates/index.html
	indexPageContentTemplate string
)
