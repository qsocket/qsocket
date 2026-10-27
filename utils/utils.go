package utils

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/qsocket/qsocket/log"
)

var (
	Red        = color.New(color.FgRed)
	Blue       = color.New(color.FgBlue)
	Yellow     = color.New(color.FgYellow)
	BoldRed    = color.New(color.FgRed).Add(color.Bold)
	BoldBlue   = color.New(color.FgBlue).Add(color.Bold)
	BoldGreen  = color.New(color.FgGreen).Add(color.Bold)
	BoldYellow = color.New(color.FgYellow).Add(color.Bold)
)

func EnableSmartPipe() {
	color.NoColor = false
	os.Stdout = os.NewFile(uintptr(syscall.Stderr), "stderr")
}

func IsFilePiped(f *os.File) bool {
	fs, err := f.Stat()
	if err != nil {
		// Processes with no console attached (e.g. Windows services) have no
		// valid standard handles to stat, hence nothing is piped.
		log.Debug(err)
		return false
	}
	return (fs.Mode() & os.ModeCharDevice) == 0
}

func PrintFatal(format string, a ...any) {
	fmt.Printf("%s ", Red.Sprintf("[!]"))
	fmt.Printf(format, a...)
}

func PrintStatus(format string, a ...any) {
	fmt.Printf("%s ", Yellow.Sprintf("[*]"))
	fmt.Printf(format, a...)
}

func PrintSuccess(format string, a ...any) {
	fmt.Printf("%s ", BoldGreen.Sprintf("[+]"))
	fmt.Printf(format, a...)
}

func CaclChecksum(data []byte, base uint) uint {
	checksum := uint(0)
	for _, n := range data {
		checksum += uint(n)
	}
	return checksum % base
}

func RandomString(n int) string {
	// rand.Seed(time.Now().UTC().UnixMicro())
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func WaitForExitSignal(sig os.Signal) {
	for {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, sig)
		<-sigChan
		print("\n")
		PrintFatal("Exiting...\n")
		os.Exit(0)
	}
}
