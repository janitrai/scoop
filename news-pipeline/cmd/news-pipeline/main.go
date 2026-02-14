package main

import (
	"os"

	"horse.fit/news-pipeline/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:]))
}
