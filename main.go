package main

import (
	"fmt"
	"os"

	"github.com/blakewilliams/goat/internal/generator"
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

					if err := generator.Compile(directory); err != nil {
						return fmt.Errorf("failed to compile: %w", err)
					}

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
