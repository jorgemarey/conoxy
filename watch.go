package main

import (
	"fmt"
	"log"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect/proxy"
	"github.com/hashicorp/consul/watch"
)

type proxyState struct {
	config    *proxy.Config
	listener  *proxy.Listener
	upstreams map[string]*proxy.Listener
}

type Config struct {
	proxyConfigs map[string]*proxy.Config
}

// ConfigWatcher is a simple interface to allow dynamic configurations from
// pluggable sources.
type ConfigWatcher interface {
	// Watch returns a channel that will deliver new Configs if something external
	// provokes it.
	Watch() <-chan *Config
}

// NodeServicesConfigWatcher watches the local Consul node for proxy service changes.
type NodeServicesConfigWatcher struct {
	client *api.Client
	logger *log.Logger
	ch     chan *Config
	plan   *watch.Plan
}

// NewNodeServicesConfigWatcher creates an AgentConfigWatcher.
func NewNodeServicesConfigWatcher(client *api.Client,
	logger *log.Logger) (*NodeServicesConfigWatcher, error) {
	w := &NodeServicesConfigWatcher{
		client: client,
		logger: logger,
		ch:     make(chan *Config),
	}

	config, err := client.Agent().Self()
	if err != nil {
		return nil, err
	}
	nodeName := config["Config"]["NodeName"].(string)

	// Setup watch plan for config
	plan, err := watch.Parse(map[string]interface{}{
		"type": "node_services",
		"node": nodeName,
	})
	if err != nil {
		return nil, err
	}
	w.plan = plan
	w.plan.HybridHandler = w.handler
	go w.plan.RunWithClientAndLogger(w.client, w.logger)
	return w, nil
}

func (w *NodeServicesConfigWatcher) handler(blockVal watch.BlockingParamVal, val interface{}) {
	resp, ok := val.(*api.CatalogNode)
	if !ok {
		w.logger.Printf("[WARN] proxy config watch returned bad response: %v", val)
		return
	}

	configs := make(map[string]*proxy.Config)
	for _, svc := range resp.Services {
		if svc.Kind != api.ServiceKindConnectProxy {
			continue
		}

		cfg := &proxy.Config{
			ProxiedServiceName:      svc.Proxy.DestinationServiceName,
			ProxiedServiceNamespace: "default",
		}

		w.logger.Printf("[WARN] SERVICE: %v", svc.Service)

		// TODO: telemetry
		// TODO: custom config (to parse it we need to base64.StdEncoding.DecodeString)

		cfg.PublicListener.BindAddress = svc.Address
		cfg.PublicListener.BindPort = svc.Port
		cfg.PublicListener.LocalServiceAddress = fmt.Sprintf("%s:%d", svc.Proxy.LocalServiceAddress, svc.Proxy.LocalServicePort)
		plcSetDefaults(&cfg.PublicListener)

		for _, u := range svc.Proxy.Upstreams {
			uc := proxy.UpstreamConfig(u)
			ucSetDefaults(&uc)

			cfg.Upstreams = append(cfg.Upstreams, uc)
		}
		configs[svc.ID] = cfg
	}
	w.ch <- &Config{
		proxyConfigs: configs,
	}
}

// Watch implements ConfigWatcher.
func (w *NodeServicesConfigWatcher) Watch() <-chan *Config {
	return w.ch
}

// Close frees watcher resources and implements io.Closer
func (w *NodeServicesConfigWatcher) Close() error {
	if w.plan != nil {
		w.plan.Stop()
	}
	return nil
}

func plcSetDefaults(plc *proxy.PublicListenerConfig) {
	if plc.LocalConnectTimeoutMs == 0 {
		plc.LocalConnectTimeoutMs = 1000
	}
	if plc.HandshakeTimeoutMs == 0 {
		plc.HandshakeTimeoutMs = 10000
	}
	if plc.BindAddress == "" {
		plc.BindAddress = "0.0.0.0"
	}
}

func ucSetDefaults(uc *proxy.UpstreamConfig) {
	if uc.DestinationType == "" {
		uc.DestinationType = "service"
	}
	if uc.DestinationNamespace == "" {
		uc.DestinationNamespace = "default"
	}
	if uc.LocalBindAddress == "" {
		uc.LocalBindAddress = "0.0.0.0"
	}
}
