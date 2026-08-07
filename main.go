package main

import (
	"context"
	"os"

	"github.com/qsocket/qs-netcat/config"
	"github.com/qsocket/qs-netcat/log"
	qsnetcat "github.com/qsocket/qs-netcat/pkg"
	"github.com/qsocket/qs-netcat/utils"
)

func main() {
	// Determine the execution mode before parsing the options, processes
	// started by the Windows service control manager have no console attached
	// so all interactive prompts needs to be disabled.
	isService := qsnetcat.IsWindowsService()
	config.NoPrompt = isService

	// Configure the options from the flags/config file
	opts, err := config.ConfigureOptions()
	if err != nil || opts == nil {
		log.Fatal(err)
		return
	}
	if opts.RandomSecret {
		opts.Secret = utils.RandomString(20)
	}

	// Hand the execution over to the SCM dispatcher when started as a service.
	if isService {
		err = qsnetcat.RunService(opts)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		return
	}

	opts.Summarize()
	if opts.Listen {
		go utils.WaitForExitSignal(os.Interrupt)
	}

	err = qsnetcat.Run(context.Background(), opts)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
