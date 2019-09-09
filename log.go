// +build windows

package winsvc

import (
	"io"
	"strings"

	"golang.org/x/sys/windows/svc/eventlog"
)

var _ io.WriteCloser = (*EventLogWriter)(nil)

type EventLogWriter struct {
	elg        *eventlog.Log
	eId        uint32
	infoKey    string
	warningKey string
	errorKey   string
}

// NewEventLogWriter creates a new io.WriteCloser.
// Writes to the returned io.WriteCloser are output to eventlog.Log.Info/Warning/Error
// depending on the presence of the keywords.
func NewEventLogWriter(
	svcname string, eId uint32, infoKey, warningKey, errorKey string,
) (*EventLogWriter, error) {
	elg, err := eventlog.Open(svcname)
	if err != nil {
		return nil, err
	}

	return &EventLogWriter{
		elg:        elg,
		eId:        eId,
		infoKey:    infoKey,
		warningKey: warningKey,
		errorKey:   errorKey,
	}, nil
}

func (elw *EventLogWriter) Close() error {
	return elw.elg.Close()
}

func (elw *EventLogWriter) Write(p []byte) (n int, err error) {
	s := string(p)
	if strings.Contains(s, elw.infoKey) {
		err = elw.elg.Info(elw.eId, s)
	} else if strings.Contains(s, elw.warningKey) {
		err = elw.elg.Warning(elw.eId, s)
	} else if strings.Contains(s, elw.errorKey) {
		err = elw.elg.Error(elw.eId, s)
	}
	return 0, err
}
