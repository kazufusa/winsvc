// +build windows

package winsvc

import (
	"time"

	"golang.org/x/sys/windows/svc/mgr"
)

const name = "test-service"

func ExampleInstallService() {
	exepath, err := SelfExePath()
	if err != nil {
		return
	}
	err = InstallService(
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
	exepath, err := SelfExePath()
	if err != nil {
		return
	}
	err = OpenPort(name, FW_RULE_DIR_IN, exepath, FW_RULE_PROTOCOL_TCP, 80)
	if err != nil {
		return
	}
}

func ExampleClosePort() {
	err := ClosePort(name)
	if err != nil {
		return
	}
}
