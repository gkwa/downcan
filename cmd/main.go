package main

import (
	"os"

	"github.com/taylormonacelli/downcan"
)

func main() {
	code := downcan.Execute()
	os.Exit(code)
}
