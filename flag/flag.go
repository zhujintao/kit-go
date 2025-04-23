package flag

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

var app *cli.Command = &cli.Command{}

func init() {

}
func Flag(name, help string) {

	f := &cli.StringFlag{
		Name:  name,
		Usage: help,
	}

	app.Flags = append(app.Flags, f)
	app.Run(context.Background(), os.Args)
	fmt.Println("--", f.Get(), "---")

}

func Parse() {

}
