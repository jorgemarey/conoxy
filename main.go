package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/consul/api"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)

	client, _ := api.NewClient(api.DefaultConfig())

	// Output this first since the config watcher below will output
	// other information.
	// c.UI.Output("Consul Connect proxy starting...")

	// Get the proper configuration watcher
	cfgWatcher, err := NewNodeServicesConfigWatcher(client, logger)
	if err != nil {
		logger.Printf("Error preparing configuration: %s", err)
		os.Exit(1)
	}

	p, err := New(client, cfgWatcher, logger)
	if err != nil {
		logger.Printf("Failed initializing proxy: %s", err)
		os.Exit(1)
	}

	// Hook the shutdownCh up to close the proxy
	go func() {
		<-MakeShutdownCh()
		p.Close()
	}()

	// // Register the service if we requested it
	// if c.register {
	// 	monitor, err := c.registerMonitor(client)
	// 	if err != nil {
	// 		c.UI.Error(fmt.Sprintf("Failed initializing registration: %s", err))
	// 		return 1
	// 	}

	// 	go monitor.Run()
	// 	defer monitor.Close()
	// }

	// c.UI.Info("")
	// c.UI.Output("Log data will now stream in as it occurs:\n")
	// logGate.Flush()

	if err := p.Serve(); err != nil {
		// c.UI.Error(fmt.Sprintf("Failed running proxy: %s", err))
	}

	// c.UI.Output("Consul Connect proxy shutdown")
}

// MakeShutdownCh returns a channel that can be used for shutdown notifications
// for commands. This channel will send a message for every interrupt or SIGTERM
// received.
func MakeShutdownCh() <-chan struct{} {
	resultCh := make(chan struct{})
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			<-signalCh
			resultCh <- struct{}{}
		}
	}()

	return resultCh
}
