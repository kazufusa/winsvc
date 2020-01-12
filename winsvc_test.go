// +build windows

package winsvc

import (
	"os"
	"time"

	"golang.org/x/sys/windows/svc/mgr"
)

func ExampleInstallService() {
	exepath, err := os.Executable()
	if err != nil {
		return
	}
	InstallService(
		exepath,
		"test-service",
		mgr.Config{
			StartType:        mgr.StartAutomatic,
			Description:      "golang test service",
			DelayedAutoStart: true,
			Dependencies:     []string{"W32time"},
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
