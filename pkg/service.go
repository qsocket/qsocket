//go:build windows
// +build windows

package qsnetcat

import (
	"context"
	"time"

	"github.com/qsocket/qs-netcat/config"
	"github.com/qsocket/qs-netcat/log"
	"golang.org/x/sys/windows/svc"
)

const (
	// SERVICE_NAME is the name registered on the SCM dispatch table.
	SERVICE_NAME = "qs-netcat"
	// SERVICE_WAIT_HINT is the estimated time (in milliseconds) reported to the
	// SCM for completing pending state changes.
	SERVICE_WAIT_HINT = 15000
	// SERVICE_STOP_TIMEOUT is the grace period given to the worker for
	// finishing up before the service reports itself as stopped.
	SERVICE_STOP_TIMEOUT = 10 * time.Second
	// SERVICE_ACCEPTED_CMDS are the SCM control codes handled by the service,
	// svc.Interrogate is always accepted by the SCM.
	SERVICE_ACCEPTED_CMDS = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
)

// Service implements the svc.Handler interface for running qs-netcat under
// the control of the Windows service control manager (SCM).
type Service struct {
	opts   *config.Options
	cancel context.CancelFunc
	done   chan error
}

// IsWindowsService reports whether the current process is being started by the
// Windows service control manager. Any error while determining the execution
// mode is treated as a regular console execution.
func IsWindowsService() bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Debug(err)
		return false
	}
	return isService
}

// RunService hands the execution over to the SCM dispatcher, it blocks until
// the service is stopped or the worker terminates on its own.
func RunService(opts *config.Options) error {
	return svc.Run(SERVICE_NAME, &Service{opts: opts})
}

// Execute is called by the SCM dispatcher on service startup, it starts the
// netcat worker and translates the incoming SCM control requests into worker
// state changes until the service is stopped.
func (s *Service) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	log.Debugf("Service arguments: %s", args)
	status <- svc.Status{State: svc.StartPending, WaitHint: SERVICE_WAIT_HINT}
	s.start()
	status <- svc.Status{State: svc.Running, Accepts: SERVICE_ACCEPTED_CMDS}
	log.Info("Service started...")

	for {
		select {
		// The worker terminated on its own, stop the service with its exit code.
		case err := <-s.done:
			s.reset()
			status <- svc.Status{State: svc.StopPending, WaitHint: SERVICE_WAIT_HINT}
			if err != nil {
				log.Error(err)
				return false, 1
			}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Infof("Received %s, stopping service...", CmdName(c.Cmd))
				status <- svc.Status{State: svc.StopPending, WaitHint: SERVICE_WAIT_HINT}
				s.stop()
				return false, 0
			case svc.Pause:
				status <- svc.Status{State: svc.PausePending, WaitHint: SERVICE_WAIT_HINT}
				s.stop()
				status <- svc.Status{State: svc.Paused, Accepts: SERVICE_ACCEPTED_CMDS}
				log.Info("Service paused...")
			case svc.Continue:
				status <- svc.Status{State: svc.ContinuePending, WaitHint: SERVICE_WAIT_HINT}
				s.start()
				status <- svc.Status{State: svc.Running, Accepts: SERVICE_ACCEPTED_CMDS}
				log.Info("Service resumed...")
			default:
				log.Debugf("Unexpected control request: #%d", c.Cmd)
			}
		}
	}
}

// start spawns the netcat worker, it is a no-op if the worker is running.
func (s *Service) start() {
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan error, 1)
	go func(done chan error) {
		done <- Run(ctx, s.opts)
	}(s.done)
}

// stop cancels the netcat worker and waits for it to return, established
// sessions are not interrupted. Probes blocking on the relay network may
// outlive the timeout, in which case they are left to the process teardown.
func (s *Service) stop() {
	if s.cancel == nil {
		return
	}
	s.cancel()
	select {
	case err := <-s.done:
		if err != nil {
			log.Debug(err)
		}
	case <-time.After(SERVICE_STOP_TIMEOUT):
		log.Debug("Timed out waiting for the worker to terminate.")
	}
	s.reset()
}

// reset releases the worker context and detaches the completion channel, a nil
// channel keeps the Execute loop from spinning while no worker is running.
func (s *Service) reset() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.done = nil
}

// CmdName returns the human readable name of a SCM control code.
func CmdName(cmd svc.Cmd) string {
	switch cmd {
	case svc.Stop:
		return "STOP"
	case svc.Pause:
		return "PAUSE"
	case svc.Continue:
		return "CONTINUE"
	case svc.Interrogate:
		return "INTERROGATE"
	case svc.Shutdown:
		return "SHUTDOWN"
	case svc.ParamChange:
		return "PARAMCHANGE"
	case svc.NetBindAdd:
		return "NETBINDADD"
	case svc.NetBindRemove:
		return "NETBINDREMOVE"
	case svc.NetBindEnable:
		return "NETBINDENABLE"
	case svc.NetBindDisable:
		return "NETBINDDISABLE"
	case svc.DeviceEvent:
		return "DEVICEEVENT"
	case svc.HardwareProfileChange:
		return "HARDWAREPROFILECHANGE"
	case svc.PowerEvent:
		return "POWEREVENT"
	case svc.SessionChange:
		return "SESSIONCHANGE"
	case svc.PreShutdown:
		return "PRESHUTDOWN"
	}
	return "UNKNOWN"
}
