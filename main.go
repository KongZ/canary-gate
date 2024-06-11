package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KongZ/canary-gate/handler"
	"github.com/KongZ/canary-gate/noti"
	"github.com/KongZ/canary-gate/store"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	defaultAddress = ":8080"

	flagVerbose          = "verbose"
	flagListenAddress    = "listen-address"
	flagSlackToken       = "slack-token"
	flagKubernetesClient = "kubernetes-client"

	kubernetesClientNone      = "none"
	kubernetesClientInCluster = "in-cluster"
)

func main() {
	cmd := &cli.Command{
		Name:        "canary-gate",
		Action:      launchServer,
		Version:     "0.1",
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
				Usage:   "Set server port. Default is :8080",
				Value:   defaultAddress,
				Sources: cli.EnvVars("LISTEN_ADDRESS"),
			},
		},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Msgf("Error: %s", err)
	}
}

func launchServer(ctx context.Context, cmd *cli.Command) error {
	switch count := cmd.Count(flagVerbose); count {
	case 1:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case 2:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	store, err := store.NewMemoryStore()
	if err != nil {
		return err
	}
	slack := noti.NewSlackClient(cmd.String(flagSlackToken))

	var kubeClient kubernetes.Interface
	if cmd.String(flagKubernetesClient) == kubernetesClientInCluster {
		log.Info().Msg("creating in-cluster kubernetes client")
		kubeConfig, err := rest.InClusterConfig()
		if err != nil {
			return err
		}
		if kubeClient, err = kubernetes.NewForConfig(kubeConfig); err != nil {
			return err
		}
	} else {
		log.Info().Msg("not creating a kubernetes client")
	}
	log.Info().Msgf("%v", kubeClient)

	listenAddress := cmd.String(flagListenAddress)
	mux := http.NewServeMux()
	mux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	handler := handler.NewHandler(cmd, slack, store)
	mux.Handle("/confirm-rollout", handler.ConfirmRollout())
	mux.Handle("/pre-rollout", handler.PreRollout())
	mux.Handle("/rollout", handler.Rollout())
	mux.Handle("/confirm-traffic-increase", handler.ConfirmTrafficIncrease())
	mux.Handle("/confirm-promotion", handler.ConfirmPromotion())
	mux.Handle("/post-rollout", handler.PostRollout())
	mux.Handle("/rollback", handler.Rollback())
	mux.Handle("/event", handler.Event())
	mux.Handle("/gate", handler.Gate())
	mux.Handle("/metrics", promhttp.Handler())
	ch := make(chan struct{})
	server := http.Server{
		Addr:              listenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGTERM)
		<-sigint
		// We received an interrupt signal, shut down.
		if err := server.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Error().Msgf("HTTP server Shutdown: %v", err)
		}
		close(ch)
	}()
	log.Info().Msgf("Listening on http://%s", listenAddress)
	return server.ListenAndServe()

	// http.Handle("/launch-test",
	// 	promhttp.InstrumentHandlerCounter(
	// 		promauto.NewCounterVec(
	// 			prometheus.CounterOpts{
	// 				Name: "launch_requests_total",
	// 				Help: "Total number of /launch-test requests by HTTP code.",
	// 			},
	// 			[]string{"code"},
	// 		),
	// 		launchHandler,
	// 	),
	// )

}
