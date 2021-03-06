package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/improbable-eng/thanos/pkg/receive"
	"github.com/oklog/run"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	config := struct {
		KubeConfig             string
		Namespace              string
		StatefulSetLabel       string
		ConfigMapName          string
		ConfigMapGeneratedName string
	}{}

	flag.StringVar(&config.KubeConfig, "kubeconfig", "", "Path to kubeconfig")
	flag.StringVar(&config.Namespace, "namespace", "default", "The namespace we operate in")
	flag.StringVar(&config.StatefulSetLabel, "statefulset-label", "controller.receive.thanos.io=thanos-receive-controller", "The label StatefulSets have to be watched with")
	flag.StringVar(&config.ConfigMapName, "configmap-name", "", "The name to the original ConfigMap containing all hashring tenants")
	flag.StringVar(&config.ConfigMapGeneratedName, "configmap-generated-name", "", "The name to the generated and populated ConfigMap")
	flag.Parse()

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = log.WithPrefix(logger, "ts", log.DefaultTimestampUTC)
	logger = log.WithPrefix(logger, "caller", log.DefaultCaller)

	konfig, err := clientcmd.BuildConfigFromFlags("", config.KubeConfig)
	if err != nil {
		stdlog.Fatal(err)
	}

	klient, err := kubernetes.NewForConfig(konfig)
	if err != nil {
		stdlog.Fatal(err)
	}

	// The entire controller is synchronized via these 2 channels.
	// The ConfigMapWatcher and StatefulSetWatcher write into individual channels.
	// Both channels are read from the XXX and merges into the populated config which is then written.

	hashringUpdates := make(chan []receive.HashringConfig)
	StatefulSetUpdates := make(chan StatefulSetUpdate)

	var gr run.Group
	{
		gr.Add(func() error {
			sig := make(chan os.Signal)
			signal.Notify(sig, os.Interrupt, os.Kill)
			<-sig
			return nil
		}, func(err error) {})
	}
	{
		ctx, cancel := context.WithCancel(context.Background())
		cmw := ConfigMapWatcher{klient: klient}

		gr.Add(func() error {
			return cmw.Watch(ctx, config.Namespace, config.ConfigMapName, hashringUpdates)
		}, func(err error) {
			level.Info(logger).Log("msg", "shutting down ConfigMap watcher")
			cancel()
		})
	}
	{
		ctx, cancel := context.WithCancel(context.Background())
		stsw := StatefulSetWatcher{klient: klient}

		gr.Add(func() error {
			return stsw.Watch(ctx, config.Namespace, config.StatefulSetLabel, StatefulSetUpdates)
		}, func(err error) {
			level.Info(logger).Log("msg", "shutting down StatefulSet watcher")
			cancel()
		})
	}
	{
		ctx, cancel := context.WithCancel(context.Background())

		cp := ConfigPopulator{
			namespace:    config.Namespace,
			statefulsets: map[string]StatefulSetUpdate{},
		}

		cms := ConfigMapSaver{
			klient:    klient,
			namespace: config.Namespace,
			name:      config.ConfigMapGeneratedName,
		}

		gr.Add(func() error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case hashring := <-hashringUpdates:
					cp.UpdateConfig(hashring)
					if err := cms.SaveHashring(cp.Populate()); err != nil {
						level.Warn(logger).Log("msg", "failed saving hashring to ConfigMap", "err", err)
						continue
					}
					level.Debug(logger).Log("msg", "Saved a newly generated ConfigMap")
				case sts := <-StatefulSetUpdates:
					cp.UpdateStatefulSet(sts)
					if err := cms.SaveHashring(cp.Populate()); err != nil {
						level.Warn(logger).Log("msg", "failed saving hashring to ConfigMap", "err", err)
						continue
					}
					level.Debug(logger).Log("msg", "Saved a newly generated ConfigMap")
				}
			}
		}, func(err error) {
			cancel()
		})
	}

	level.Info(logger).Log("msg", "starting the controller")

	if err := gr.Run(); err != nil {
		stdlog.Fatal(err)
	}
}

type ConfigPopulator struct {
	namespace    string
	hashrings    []receive.HashringConfig
	statefulsets map[string]StatefulSetUpdate

	config []receive.HashringConfig
}

func (cp *ConfigPopulator) UpdateConfig(hashrings []receive.HashringConfig) {
	cp.hashrings = hashrings
}

func (cp *ConfigPopulator) UpdateStatefulSet(sts StatefulSetUpdate) {
	if cp.statefulsets == nil {
		cp.statefulsets = make(map[string]StatefulSetUpdate, 1)
	}

	cp.statefulsets[sts.Name] = sts
}

func (cp *ConfigPopulator) Populate() []receive.HashringConfig {
	hashrings := cp.hashrings

	for i, hashring := range hashrings {
		if sts, exists := cp.statefulsets[hashring.Hashring]; exists {
			var endpoints []string
			for i := 0; i < sts.Replicas; i++ {
				endpoints = append(endpoints,
					// TODO: Make sure this is actually correct
					fmt.Sprintf("%s://%s-%d.%s.%s:%d",
						"https",
						sts.Name,
						i,
						sts.Name,
						cp.namespace,
						10901,
					),
				)
			}
			hashrings[i].Endpoints = endpoints
		}
	}

	return hashrings
}
