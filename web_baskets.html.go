package main

import _ "embed"

var (
	//go:embed templates/baskets.html
	basketsPageContentTemplate string
)
