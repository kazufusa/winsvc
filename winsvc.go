// +build windows

package winsvc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

var StateLabel = map[svc.State]string{
	svc.Stopped:         "Stopped",
	svc.StartPending:    "StartPending",
	svc.StopPending:     "StopPending",
	svc.Running:         "Running",
	svc.ContinuePending: "ContinuePending",
	svc.PausePending:    "PausePending",
	svc.Paused:          "Paused",
}

// SvcOpt can apply SetRecoveryActions, SetRecoveryCommand or SetRebootMessage
// to service.
type SvcOpt func(*mgr.Service) error

// InstallService installs new service name on the system.
func InstallService(exepath, name string, mgrConfig mgr.Config, cmdArgs []string, opts ...SvcOpt) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
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
	defer m.Disconnect()
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
	defer m.Disconnect()
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

// ControlService sends state change request c to the service name.
func ControlService(ctx context.Context, name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}

	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			status, err = s.Query()
			if status.State == to {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// QueryService gets the status of the service name
func QueryService(name string) (svc.State, error) {
	m, err := mgr.Connect()
	if err != nil {
		return 0, err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return 0, fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Query()
	if err != nil {
		return 0, err
	}

	return status.State, nil
}

// GetTimeout returns the timeout duration based on the registry value of
// LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\WaitToKillServiceTimeout
func GetTimeout() (d time.Duration, err error) {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return
	}
	defer key.Close()
	sv, _, err := key.GetStringValue("WaitToKillServiceTimeout")
	if err != nil {
		return
	}
	v, err := strconv.Atoi(sv)
	if err != nil {
		return
	}
	d = time.Duration(v) * time.Millisecond
	return
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
