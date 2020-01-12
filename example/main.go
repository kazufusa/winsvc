// +build windows

// Example service program that works as http server.
//
// The program demonstrates how to create Windows service with winsvc.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
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
	var err error
	timeout, err = winsvc.GetTimeout()
	if err != nil {
		timeout = 5 * time.Second
	}

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

		exepath, err = os.Executable()
		if err != nil {
			break
		}
		err = winsvc.InstallService(
			syscall.EscapeArg(exepath),
			name,
			mgr.Config{
				StartType:        mgr.StartAutomatic,
				Description:      "golang test service",
				DelayedAutoStart: true,
				Dependencies:     []string{"W32time"},
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
		err = OpenPort(name, FW_RULE_DIR_IN, exepath, FW_RULE_PROTOCOL_TCP, 80)
	case "remove":
		err = winsvc.RemoveService(name)
		if err != nil {
			break
		}
		err = ClosePort(name)
	case "start":
		err = winsvc.StartService(name, nil)
	case "stop":
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err = winsvc.ControlService(ctx, name, svc.Stop, svc.Stopped)
	case "pause":
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err = winsvc.ControlService(ctx, name, svc.Pause, svc.Paused)
	case "continue":
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err = winsvc.ControlService(ctx, name, svc.Continue, svc.Running)
	case "status":
		var status svc.State
		status, err = winsvc.QueryService(name)
		if err == nil {
			fmt.Printf("%s %s\r\n", name, winsvc.StateLabel[status])
			return
		}
	default:
		usage(fmt.Sprintf("invalid command %s", cmd))
	}
	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, name, err)
	}
}

const (
	FW_RULE_DIR_IN       = "in"
	FW_RULE_DIR_OUT      = "out"
	FW_RULE_PROTOCOL_TCP = "TCP"
	FW_RULE_PROTOCOL_UDP = "UDP"
)

// OpenPort creates a new rule to allow the port on the system firewall
func OpenPort(name, dir, program, protocol string, port int) error {
	err := exec.Command(
		"netsh",
		"advfirewall",
		"firewall",
		"add",
		"rule",
		fmt.Sprintf("name=%s", syscall.EscapeArg(name)),
		fmt.Sprintf("dir=%s", dir),
		"action=allow",
		fmt.Sprintf("program=%s", syscall.EscapeArg(program)),
		fmt.Sprintf("protocol=%s", protocol),
		fmt.Sprintf("localport=%d", port),
		"enable=yes",
	).Run()
	if err != nil {
		return fmt.Errorf("error in firewall hanlding: %s", err)
	}
	return nil
}

// ClosePort deletes a rule on the system firewall
func ClosePort(name string) error {
	err := exec.Command(
		"netsh",
		"advfirewall",
		"firewall",
		"delete",
		"rule",
		fmt.Sprintf("name=%s", syscall.EscapeArg(name)),
	).Run()
	if err != nil {
		return fmt.Errorf("error in firewall hanlding: %s", err)
	}
	return nil
}
