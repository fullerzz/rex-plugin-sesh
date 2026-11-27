package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fullerzz/rex-plugin-sesh/internal/app"
)

func main() {
	if err := app.New().Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
