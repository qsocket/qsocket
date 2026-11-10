package main

import (
	"context"
	"errors"
	"os"

	"github.com/qsocket/qsocket/config"
	"github.com/qsocket/qsocket/log"
	qsock "github.com/qsocket/qsocket/pkg"
	"github.com/qsocket/qsocket/utils"
)

func main() {
	// Determine the execution mode before parsing the options, processes
	// started by the Windows service control manager have no console attached
	// so all interactive prompts needs to be disabled.
	isService := qsock.IsWindowsService()
	config.NoPrompt = isService

	// Configure the options from the command line
	opts, err := config.ConfigureOptions()
	if err != nil || opts == nil {
		log.Fatal(err)
		return
	}

	// Hand the execution over to the SCM dispatcher when started as a service.
	if isService {
		if err := qsock.RunService(opts); err != nil {
			log.Error(err)
			os.Exit(1)
		}
		return
	}

	opts.Summarize()
	if opts.Listen {
		go utils.WaitForExitSignal(os.Interrupt)
	}

	if err := qsock.Run(context.Background(), opts); err != nil {
		// The subcommands reporting their own failures return a silent
		// error, printing it again would only repeat the diagnostics.
		if !errors.Is(err, qsock.ErrSilent) {
			log.Error(err)
		}
		os.Exit(1)
	}
}
