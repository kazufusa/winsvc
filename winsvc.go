// +build windows

package winsvc

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// SvcOpt can apply SetRecoveryActions, SetRecoveryCommand or SetRebootMessage
// to service.
type SvcOpt func(*mgr.Service) error

// InstallService installs new service name on the system.
func InstallService(exepath, name string, mgrConfig mgr.Config, cmdArgs []string, opts ...SvcOpt) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Disconnect(); err != nil {
			log.Printf("failed to disconnect system service manager: %s", err)
		}
	}()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	s, err = m.CreateService(name, exepath, mgrConfig, cmdArgs...)
	if err != nil {
		return err
	}
	defer s.Close()
	for _, opt := range opts {
		err := opt(s)
		if err != nil {
			return err
		}
	}
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

// RemoveService removes the service name on the system.
func RemoveService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Disconnect(); err != nil {
			log.Printf("failed to disconnect system service manager: %s", err)
		}
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}

// StartService starts the service name
func StartService(name string, cmdArgs []string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Disconnect(); err != nil {
			log.Printf("failed to disconnect system service manager: %s", err)
		}
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start(cmdArgs...)
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

// ControlService  sends state change request c to the service name.
func ControlService(name string, c svc.Cmd, to svc.State, timeout time.Duration) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Disconnect(); err != nil {
			log.Printf("failed to disconnect system service manager: %s", err)
		}
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeLimit := time.Now().Add(timeout)
	for status.State != to {
		if timeLimit.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

// SelfExePath creates filepath string of self execution binary
func SelfExePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
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

// GetTimeout returns the timeout duration based on the registry value of
// LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\WaitToKillServiceTimeout
func GetTimeout() time.Duration {
	defDu := time.Duration(5 * time.Second)
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return defDu
	}
	defer key.Close()
	sv, _, err := key.GetStringValue("WaitToKillServiceTimeout")
	if err != nil {
		return defDu
	}
	v, err := strconv.Atoi(sv)
	if err != nil {
		return defDu
	}
	return time.Duration(v) * time.Millisecond
}

// MaximizeTimeout tries to change the registry value of
// LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\WaitToKillServiceTimeout
// to "20000"
func MaximizeTimeout() error {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control`,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer key.Close()
	sv, _, err := key.GetStringValue("WaitToKillServiceTimeout")
	if err != nil {
		return err
	}
	if sv == "20000" {
		return nil
	}
	return key.SetStringValue("WaitToKillServiceTimeout", "20000")
}
