package main

import _ "embed"

var (
	//go:embed templates/basket.html
	basketPageContentTemplate string
)
