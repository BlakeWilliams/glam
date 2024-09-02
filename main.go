package main

import (
	"fmt"
	"os"

	"github.com/blakewilliams/goat/internal/compiler"
	"github.com/urfave/cli"
)

func main() {
	app := &cli.App{
		Name:  "goat",
		Usage: "GOAT generates components from Go structs",
		Commands: []cli.Command{
			{
				Name:    "generate",
				Aliases: []string{"g"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "directory",
						Usage: "The directory to create generate component code for",
					},
				},

				Action: func(c *cli.Context) error {
					directory := c.Args().First()
					if directory == "" {
						return fmt.Errorf("directory is required")
					}

					if err := compiler.Compile(directory); err != nil {
						return fmt.Errorf("failed to compile: %w", err)
					}
					fmt.Println("OK")

					return nil
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		defer func() {
			os.Exit(1)
		}()
	}
}
