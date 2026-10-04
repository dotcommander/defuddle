package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
)

type cli struct {
	Report  reportCmd  `cmd:"" help:"Compare a saved HTML manifest."`
	Adapter adapterCmd `cmd:"" help:"Serve the upstream JSONL extractor protocol."`
}

type reportCmd struct {
	Manifest string `arg:"" type:"path" help:"JSON fixture manifest."`
	Output   string `short:"o" default:"-" help:"JSONL output path or -."`
}

func (c *reportCmd) Run(ctx context.Context, output io.Writer) error {
	fixtures, err := loadManifest(c.Manifest)
	if err != nil {
		return err
	}
	if c.Output != "-" {
		f, err := os.Create(c.Output)
		if err != nil {
			return err
		}
		defer f.Close()
		output = f
	}
	return writeReport(ctx, fixtures, output)
}

type adapterCmd struct {
	Engine string `required:"" enum:"defuddle,trafilatura,readability" help:"Extraction engine."`
}

func (c *adapterCmd) Run(ctx context.Context, output io.Writer) error {
	return runAdapter(ctx, c.Engine, os.Stdin, output)
}

func run(args []string, stdout, stderr io.Writer) error {
	var command cli
	parser, err := kong.New(&command, kong.Name("compare"), kong.Writers(stdout, stderr), kong.BindTo(context.Background(), (*context.Context)(nil)), kong.BindTo(stdout, (*io.Writer)(nil)))
	if err != nil {
		return err
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	return ctx.Run()
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
