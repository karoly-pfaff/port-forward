//go:build windows

package platform

import (
	"context"

	"golang.org/x/sys/windows/svc"
)

// IsWindowsService reports true when running under the Windows Service Control Manager.
func IsWindowsService() bool {
	ok, _ := svc.IsWindowsService()
	return ok
}

// RunAsWindowsService registers with the SCM and runs until a Stop or Shutdown
// control is received. run receives a context that is cancelled on stop.
func RunAsWindowsService(name string, run func(ctx context.Context) error) error {
	return svc.Run(name, &serviceHandler{run: run})
}

type serviceHandler struct {
	run func(ctx context.Context) error
}

func (h *serviceHandler) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- h.run(ctx)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			}
		case <-runDone:
			break loop
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	cancel()
	<-runDone
	return false, 0
}
