package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/gabriel-samfira/coriolis-logger/writers/stdout"
	"github.com/gabriel-samfira/coriolis-logger/writers/websocket"

	"github.com/gabriel-samfira/coriolis-logger/config"
	"github.com/gabriel-samfira/coriolis-logger/datastore"
	"github.com/gabriel-samfira/coriolis-logger/logging"
	"github.com/gabriel-samfira/coriolis-logger/syslog"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("coriolis.cmd.logger")

func main() {
	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGTERM)
	signal.Notify(stop, syscall.SIGINT)
	log.SetLogLevel(loggo.DEBUG)

	cfgFile := flag.String("config", "", "coriolis-logger config file")
	flag.Parse()

	if *cfgFile == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	cfg, err := config.NewConfig(*cfgFile)
	if err != nil {
		log.Errorf("error reading config: %q", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		log.Errorf("failed to validate config: %q", err)
		os.Exit(1)
	}
	// ctx, cancel := context.WithCancel(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error)

	datastore, err := datastore.GetDatastore(cfg.Syslog)
	if err != nil {
		log.Errorf("error getting datastore: %q", err)
		os.Exit(1)
	}
	stdoutWriter, err := stdout.NewStdOutWriter()
	if err != nil {
		log.Errorf("error getting stdout datastore: %q", err)
		os.Exit(1)
	}

	websocketWorker := websocket.NewHub(ctx)
	if err := websocketWorker.Start(); err != nil {
		log.Errorf("error starting websocket worker: %q", err)
		os.Exit(1)
	}

	writer := logging.NewAggregateWriter(datastore, stdoutWriter, websocketWorker)
	syslogSvc, err := syslog.NewSyslogServer(ctx, cfg.Syslog, writer, errChan)
	if err != nil {
		log.Errorf("error getting syslog worker: %q", err)
		os.Exit(1)
	}
	if err := syslogSvc.Start(); err != nil {
		log.Errorf("error starting syslog worker: %q", err)
		os.Exit(1)
	}

	select {
	case <-stop:
		log.Infof("shutting down gracefully")
		// if err := syslogSvc.Stop(); err != nil {
		// 	log.Errorf("error stopping syslog worker: %q", err)
		// }
		cancel()
	case err := <-errChan:
		log.Errorf("worker set error: %q. Shutting down", err)
		// if err := syslogSvc.Stop(); err != nil {
		// 	log.Errorf("error stopping syslog worker: %q", err)
		// }
		cancel()
	}
	syslogSvc.Wait()
}
