package main

import (
	"errors"
	"flag"
	"io"

	"github.com/AlexandreSJ/noct/internal/logstore"
)

// printLogTimes is noct log-times [--from N] [--to N] [--follow] FILE...: the log's lines, each
// after when it started, for the logs view. It is not in the help: noct runs it for itself.
func printLogTimes(args []string, out io.Writer) error {
	var r logstore.LineRange
	set := flag.NewFlagSet("log-times", flag.ContinueOnError)
	set.IntVar(&r.From, "from", 0, "first line")
	set.IntVar(&r.To, "to", 0, "last line")
	set.BoolVar(&r.Follow, "follow", false, "go on with what the last file gets")
	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() == 0 {
		return errors.New("usage: noct log-times [--from N] [--to N] [--follow] FILE...")
	}
	return logstore.PrintLines(out, r, nil, set.Args()...)
}
