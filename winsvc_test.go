// +build windows

package winsvc

import (
	"time"

	"golang.org/x/sys/windows/svc/mgr"
)

func ExampleInstallService() {
	winsvc.MaximizeTimeout()
	exepath, err := winsvc.SelfExePath()
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
		nil, // command-line arguments when the service is started
		func(s *mgr.Service) error { // enable to apply SetRecoveryActions, SetRecoveryCommand or SetRebootMessage
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
}

func ExampleOpenPort() {
	err = winsvc.OpenPort(name, winsvc.FW_RULE_DIR_IN, exepath, winsvc.FW_RULE_PROTOCOL_TCP, 80)
}

func ExampleClosePort() {
	err = winsvc.ClosePort(name)
}
