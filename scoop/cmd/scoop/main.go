package main

import (
	"os"

	"horse.fit/scoop/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:]))
}
