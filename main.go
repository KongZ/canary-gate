/*
Copyright 2025 The canary-gate authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KongZ/canary-gate/handler"
	"github.com/KongZ/canary-gate/noti"
	"github.com/KongZ/canary-gate/store"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog"

	flaggerv1beta1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	piggysecv1alpha1 "github.com/KongZ/canary-gate/api/v1alpha1"
	"github.com/KongZ/canary-gate/controller"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

const (
	defaultAddress           = ":8080"
	defaultControllerAddress = ":8081"
	defaultMetricsAddress    = ":9090"

	flagVerbose           = "verbose"
	flagListenAddress     = "listen-address"
	flagControllerAddress = "controller-address"
	flagMetricsAddress    = "metrics-address"
	flagSlackToken        = "slack-token"
	flagSlackChannel      = "slack-channel"
	flagKubernetesClient  = "kubernetes-client"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(piggysecv1alpha1.AddToScheme(scheme))
	utilruntime.Must(flaggerv1beta1.AddToScheme(scheme))
}

// main is the entry point for the Canary Gate application.
func main() {
	cmd := &cli.Command{
		Name:        "canary-gate",
		Action:      launchServer,
		Usage:       "Launches Canary Gate for Flagger",
		HideVersion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    flagVerbose,
				Aliases: []string{"v"},
				Usage:   "Print debugging messages. Multiple -v options increase the verbosity. The maximum is 2.",
				Value:   false,
				Sources: cli.EnvVars("VERBOSE"),
			},
			&cli.StringFlag{
				Name:    flagListenAddress,
				Usage:   fmt.Sprintf("Set server port. Default is %s", defaultAddress),
				Value:   defaultAddress,
				Sources: cli.EnvVars("LISTEN_ADDRESS"),
			},
			&cli.StringFlag{
				Name:    flagControllerAddress,
				Usage:   fmt.Sprintf("Set controller port. Default is %s", defaultControllerAddress),
				Value:   defaultControllerAddress,
				Sources: cli.EnvVars("LISTEN_CONTROLLER_ADDRESS"),
			},
			&cli.StringFlag{
				Name:    flagMetricsAddress,
				Usage:   fmt.Sprintf("Set metrics port. Default is %s", defaultMetricsAddress),
				Value:   defaultMetricsAddress,
				Sources: cli.EnvVars("LISTEN_METRICS_ADDRESS"),
			},
			&cli.StringFlag{
				Name:    flagSlackToken,
				Usage:   "Set Slack Bot User OAuth Token",
				Value:   "",
				Sources: cli.EnvVars("SLACK_TOKEN"),
				Hidden:  true, // Slack integration is not completely implemented yet
			},
			&cli.StringFlag{
				Name:    flagSlackChannel,
				Usage:   "Set Slack Channel",
				Value:   "",
				Sources: cli.EnvVars("SLACK_CHANNEL"),
				Hidden:  true, // Slack integration is not completely implemented yet
			},
		},
	}
	ctx := ctrl.SetupSignalHandler()
	if err := cmd.Run(ctx, os.Args); err != nil {
		log.Fatal().Msgf("Error: %s", err)
	}
}

// launchController starts the controller manager with the specified health checks.
func launchController(ctx context.Context, cmd *cli.Command, livez, readyz healthz.Checker) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: cmd.String(flagControllerAddress),
		Metrics: metricsserver.Options{
			BindAddress: cmd.String(flagMetricsAddress),
		},
		LeaderElection:   true,
		LeaderElectionID: "9f9b5a17.piggysec.com",
	})
	if err != nil {
		log.Fatal().Msgf("Unable to start controller: %s", err)
	}
	if err = (&controller.CanaryGateReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("canary-gate-controller"),
	}).SetupWithManager(mgr); err != nil {
		log.Fatal().Msgf("Unable to create controller: %s", err)
	}

	// Setup built-in manager health and ready checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Fatal().Msgf("Unable to set up health check: %s", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Fatal().Msgf("Unable to set up ready check: %s", err)
	}

	// Add custom health and ready checks
	if err := mgr.AddHealthzCheck("app-healthz", livez); err != nil {
		log.Fatal().Msgf("Unable to set up health check: %s", err)
	}
	if err := mgr.AddReadyzCheck("app-readyz", readyz); err != nil {
		log.Fatal().Msgf("Unable to set up ready check: %s", err)
	}

	log.Info().Msgf("Starting controller")
	if err := mgr.Start(ctx); err != nil {
		log.Fatal().Msgf("Problem running controller: %s", err)
	}
}

// appHealthz is a health check function for the application.
func appHealthz(r *http.Request) error {
	// app health check always returns nil, indicating the application is healthy.
	return nil
}

// launchServer starts the HTTP server for Canary Gate.
func launchServer(ctx context.Context, cmd *cli.Command) error {
	switch count := cmd.Count(flagVerbose); count {
	case 1:
		log.Info().Str("level", "debug").Msg("Set log level to [debug]")
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		// disable klog output
		klog.SetOutput(io.Discard)
		klog.InitFlags(nil)
		// diable controller-runtime logging
		ctrl.SetLogger(logr.New(ctrllog.NullLogSink{}))
	case 2:
		log.Info().Str("level", "trace").Msg("Set log level to [trace]")
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
		// enable controller-runtime logging
		ctrl.SetLogger(logr.New(&controller.LogrAdapter{}))
	default:
		log.Info().Str("level", "info").Msg("Set log level to [info]")
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		// disable klog output
		klog.SetOutput(io.Discard)
		klog.InitFlags(nil)
		// diable controller-runtime logging
		ctrl.SetLogger(logr.New(ctrllog.NullLogSink{}))
	}

	var stor store.Store
	var err error

	storeName := os.Getenv("CANARY_GATE_STORE")
	switch storeName {
	case "configmap":
		stor, err = store.NewConfigMapStore(nil)
	case "memory":
		stor, err = store.NewMemoryStore()
	default:
		stor, err = store.NewCanaryGateStore(nil)
	}
	if err != nil {
		return err
	}

	slack := noti.NewSlackClient(noti.SlackOption{
		Token:   cmd.String(flagSlackToken),
		Channel: cmd.String(flagSlackChannel),
	})

	listenAddress := cmd.String(flagListenAddress)
	mux := http.NewServeMux()
	serverHandler := handler.ServerHandler{}
	handler := handler.NewHandler(cmd, slack, stor)
	mux.Handle("/confirm-rollout", handler.ConfirmRollout())
	mux.Handle("/pre-rollout", handler.PreRollout())
	mux.Handle("/rollout", handler.Rollout())
	mux.Handle("/confirm-traffic-increase", handler.ConfirmTrafficIncrease())
	mux.Handle("/confirm-promotion", handler.ConfirmPromotion())
	mux.Handle("/post-rollout", handler.PostRollout())
	mux.Handle("/rollback", handler.Rollback())
	mux.Handle("/event", handler.Event())
	mux.Handle("/open", handler.OpenGate())
	mux.Handle("/close", handler.CloseGate())
	mux.Handle("/status", handler.StatusGate())
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/version", serverHandler.Version())
	// Note: The health check endpoints are merged with the controller manager.
	ch := make(chan struct{})
	server := http.Server{
		Addr:              listenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}

	// start controller
	go launchController(ctx, cmd, appHealthz, appHealthz)

	// start server
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGTERM)
		<-sigint
		// We received an interrupt signal, shut down.
		if err := stor.Shutdown(); err != nil {
			log.Error().Msgf("Store Shutdown: %v", err)
		}
		if err := server.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Error().Msgf("HTTP server Shutdown: %v", err)
		}
		close(ch)
	}()
	log.Info().Msgf("Listening on http://%s", listenAddress)
	return server.ListenAndServe()
}
