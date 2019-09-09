// +build windows

// Example service program that works as http server.
//
// The program demonstrates how to create Windows service with winsvc.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kazufusa/winsvc"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/mgr"
)

const name = "__winsvc"

var (
	timeout time.Duration
)

func init() {
	timeout = winsvc.GetTimeout()
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func usage(errmsg string) {
	fmt.Fprintf(os.Stderr,
		"%s\n\n"+
			"usage: %s <command>\n"+
			"       where <command> is one of install, remove, debug, start or stop.\n",
		errmsg, os.Args[0])
	os.Exit(1)
}

func Run(name string, isDebug bool) {
	var err error
	run := svc.Run
	w, err := winsvc.NewEventLogWriter(name, 1, "[INFO]", "[WARN]", "[ERROR]")
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()
	if isDebug {
		run = debug.Run
		log.SetOutput(io.MultiWriter(os.Stdout, w))
	} else {
		log.SetOutput(w)
	}

	log.Printf("[INFO] starting %s service", name)
	err = run(name, &myService{timeout: timeout})
	if err != nil {
		log.Printf("[ERROR] %s service failed: %v", name, err)
		return
	}
	log.Printf("[INFO] %s service stopped", name)
}

func main() {
	isIntSess, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Fatalf("failed to determine if we are running in an interactive session: %v", err)
	}
	if !isIntSess {
		Run(name, false)
		return
	}

	if len(os.Args) < 2 {
		usage("no command specified")
	}
	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
		Run(name, true)
		return
	case "install":
		var exepath string
		// err = winsvc.MaximizeTimeout()
		// if err == nil {
		// 	timeout = winsvc.GetTimeout()
		// }

		exepath, err = winsvc.SelfExePath()
		if err != nil {
			break
		}
		err = winsvc.InstallService(
			exepath,
			name,
			mgr.Config{
				StartType:   mgr.StartAutomatic,
				Description: "golang test service",
			},
			nil,
			func(s *mgr.Service) error {
				return s.SetRecoveryActions(
					[]mgr.RecoveryAction{
						mgr.RecoveryAction{
							Type:  mgr.ServiceRestart,
							Delay: 1 * time.Minute,
						},
						mgr.RecoveryAction{
							Type:  mgr.ServiceRestart,
							Delay: 1 * time.Minute,
						},
						mgr.RecoveryAction{
							Type:  mgr.ServiceRestart,
							Delay: 1 * time.Minute,
						},
					}, 0,
				)
			},
		)
		if err != nil {
			break
		}
		err = winsvc.OpenPort(name, winsvc.FW_RULE_DIR_IN, exepath, winsvc.FW_RULE_PROTOCOL_TCP, 80)
	case "remove":
		err = winsvc.RemoveService(name)
		if err != nil {
			break
		}
		err = winsvc.ClosePort(name)
	case "start":
		err = winsvc.StartService(name, nil)
	case "stop":
		err = winsvc.ControlService(name, svc.Stop, svc.Stopped, timeout)
	case "pause":
		err = winsvc.ControlService(name, svc.Pause, svc.Paused, timeout)
	case "continue":
		err = winsvc.ControlService(name, svc.Continue, svc.Running, timeout)
	default:
		usage(fmt.Sprintf("invalid command %s", cmd))
	}
	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, name, err)
	}
}
