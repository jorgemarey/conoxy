package main

import (
	"crypto/x509"
	"log"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/connect/proxy"
	"github.com/hashicorp/consul/lib"
)

// Proxy implements the built-in connect proxy.
type Proxy struct {
	client      *api.Client
	cfgWatcher  ConfigWatcher
	stopChan    chan struct{}
	logger      *log.Logger
	services    map[string]*connect.Service // the key is the serviceName (could use the same service for several instances)
	proxyStates map[string]*proxyState      // the key is the serviceID (upstreams and public listener are unique)
}

// New returns a proxy with the given configuration source.
//
// The ConfigWatcher can be used to update the configuration of the proxy.
// Whenever a new configuration is detected, the proxy will reconfigure itself.
func New(client *api.Client, cw ConfigWatcher, logger *log.Logger) (*Proxy, error) {
	return &Proxy{
		client:      client,
		cfgWatcher:  cw,
		stopChan:    make(chan struct{}),
		logger:      logger,
		services:    make(map[string]*connect.Service),
		proxyStates: make(map[string]*proxyState),
	}, nil
}

// Serve the proxy instance until a fatal error occurs or proxy is closed.
func (p *Proxy) Serve() error {
	for {
		select {
		case cfg := <-p.cfgWatcher.Watch():
			p.logger.Printf("[DEBUG] got new config")
			for id, config := range cfg.proxyConfigs {
				p.runServiceProxy(id, config)
			}
			p.removeNotPresent(cfg.proxyConfigs)
		case <-p.stopChan:
			return nil
		}
	}
}

func (p *Proxy) runServiceProxy(id string, cfg *proxy.Config) {
	if _, ok := p.proxyStates[id]; ok {
		return
	}
	p.proxyStates[id] = &proxyState{
		config:    cfg,
		upstreams: make(map[string]*proxy.Listener),
	}
	var err error
	svc, ok := p.services[cfg.ProxiedServiceName]
	if !ok {
		svc, err = cfg.Service(p.client, p.logger)
		if err != nil {
			p.logger.Printf("[ERROR] cannot create service: %s", err)
		}
		p.services[cfg.ProxiedServiceName] = svc

		if _, err = lib.InitTelemetry(cfg.Telemetry); err != nil {
			p.logger.Printf("[ERR] proxy telemetry config error: %s", err)
		}

		waitSVC(svc)
		p.logger.Printf("[INFO] Proxy config changed and ready to serve")
		tcfg := svc.ServerTLSConfig()
		cert, _ := tcfg.GetCertificate(nil)
		leaf, _ := x509.ParseCertificate(cert.Certificate[0])
		p.logger.Printf("[INFO] TLS Identity: %s", leaf.URIs[0])
		roots, err := connect.CommonNamesFromCertPool(tcfg.RootCAs)
		if err != nil {
			p.logger.Printf("[ERR] Failed to parse root subjects: %s", err)
		} else {
			p.logger.Printf("[INFO] TLS Roots   : %v", roots)
		}
	}

	go func() {
		waitSVC(svc)
		// TODO: Only launch this if there's a public listener configured
		l := proxy.NewPublicListener(svc, cfg.PublicListener, p.logger)
		if err = p.startListener("public listener", l); err != nil {
			p.logger.Printf("[ERR] failed to start public listener: %s", err)
		}
		p.proxyStates[id].listener = l
	}()

	for _, uc := range cfg.Upstreams {
		l := proxy.NewUpstreamListener(svc, p.client, uc, p.logger)
		if err = p.startListener(uc.String(), l); err != nil {
			p.logger.Printf("[ERR] failed to start upstream %s: %s", uc.String(),
				err)
		}
		p.proxyStates[id].upstreams[uc.String()] = l
	}
}

// startPublicListener is run from the internal state machine loop
func (p *Proxy) startListener(name string, l *proxy.Listener) error {
	p.logger.Printf("[INFO] %s starting on %s", name, l.BindAddr())
	go func() {
		if err := l.Serve(); err != nil {
			p.logger.Printf("[ERR] %s stopped with error: %s", name, err)
			return
		}
		p.logger.Printf("[INFO] %s stopped", name)
	}()

	go func() {
		<-p.stopChan
		l.Close()
	}()
	return nil
}

func (p *Proxy) removedProxies(new map[string]*proxy.Config) map[string]*proxyState {
	sc := make(map[string]*proxyState)
	for k, v := range p.proxyStates {
		sc[k] = v
	}
	for k := range new {
		delete(sc, k)
	}
	return sc
}

func (p *Proxy) removedServices(new map[string]*proxy.Config) map[string]*connect.Service {
	sc := make(map[string]*connect.Service)
	for k, v := range p.services {
		sc[k] = v
	}
	for _, v := range new {
		delete(sc, v.ProxiedServiceName)
	}
	return sc
}

func (p *Proxy) removeNotPresent(new map[string]*proxy.Config) {
	for id, state := range p.removedProxies(new) {
		// TODO: only stop if set (could be a service without a port)
		p.logger.Printf("[INFO] listener for service %s is going to be stop and removed", id)
		state.listener.Close()
		for _, u := range state.upstreams {
			u.Close()
		}
		delete(p.proxyStates, id)
	}
	for name, service := range p.removedServices(new) {
		p.logger.Printf("[INFO] service %s is going to be stop and removed", name)
		service.Close()
		delete(p.services, name)
	}
}

// Close stops the proxy and terminates all active connections. It must be
// called only once.
func (p *Proxy) Close() {
	close(p.stopChan)
	for _, svc := range p.services {
		svc.Close()
	}
}

func waitSVC(svc *connect.Service) {
	ch := svc.ReadyWait()
	if ch == nil {
		return
	}
	<-ch
}
