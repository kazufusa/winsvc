// +build windows

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/sys/windows/svc"
)

type feature struct {
	httpsrv *http.Server
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] %s %s %s\r\n", r.RemoteAddr, r.Method, r.URL)
	fmt.Fprintf(w, "hello world")
}

func (f *feature) Start() (<-chan struct{}, error) {
	quit := make(chan struct{})
	f.httpsrv = &http.Server{Addr: ":80", Handler: http.HandlerFunc(handler)}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[ERROR] error on http server: %+v\r\n", err)
			}
			close(quit)
		}()

		if err := f.httpsrv.ListenAndServe(); err != nil {
			log.Printf("[INFO] %s\r\n", err)
		}
	}()
	return quit, nil
}

func (f *feature) Shutdown(ctx context.Context) (err error) {
	if err = f.httpsrv.Shutdown(ctx); err != nil {
		return err
	} else {
		return nil
	}
}

var _ svc.Handler = (*myService)(nil)

type myService struct {
	timeout time.Duration
}

func (s *myService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	changes <- svc.Status{State: svc.StartPending}
	f := feature{}
	quit, err := f.Start()
	if err != nil {
		log.Printf("[ERROR] failed to start service\r\n")
		changes <- svc.Status{State: svc.StopPending}
		return
	}
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

loop:
	for {
		select {
		case <-quit:
			log.Printf("[ERROR] service job stopped\r\n")
			break loop
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
				defer cancel()
				if err := f.Shutdown(ctx); err != nil {
					log.Printf("[ERROR] failed to shutdown: %s\r\n", err)
				} else {
					log.Printf("[INFO] succeeded to gracefull shutdown http server\r\n")
				}
				break loop
			default:
				log.Printf("[ERROR] unexpected control request #%d\r\n", c)
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}
