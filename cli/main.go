package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/KongZ/canary-gate/handler"
	"github.com/KongZ/canary-gate/service"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// main is the entry point of the application.
func main() {
	// Setup structured, human-friendly logging.
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Create and run the CLI application.
	app := createCliApp()
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Err(err).Msg("Application failed")
	}
}

const diagram = "   .─.        ┌───────────────┐                                 ┌──────────┐                     \n  (   )──────▶│confirm-rollout│───────open─────────────────────▶│ rollout  │◀───────┐            \n   `─'        └───────────────┘                 ┌──close────────└──────────┘        │            \n  deploy              │                         │                     │             │            \n                    close                       ▼                     │             │            \n                      │                        .─.                  open            │            \n                      ▼                       (   )                   │             │            \n                     .─.                       `─'                    ▼             │            \n                    (   )                     pause                  .─.            │            \n                     `─'     ┌──────────────────────────────────────(   )           │            \n                    pause    │                                       `─'            │            \n                           errors                                   check          .─.           \n                             │                                     metrics        (   ) increase \n                             │                                        │            `─'  traffic  \n                             │                                        │             ▲            \n                             │                                        ▼             │            \n                             │                               ┌────────────────┐     │            \n                             │            .─.                │confirm-traffic-│     │            \n                             │           (   )◀────close─────│    increase    │     │            \n                             │            `─'                └────────────────┘     │            \n                             │           pause                        │           close          \n                             │                                      open            │            \n                             │                                        │             │            \n                             ▼                                        ▼             │            \n                            .─.                                ┌────────────┐       │            \n                 rollback  (███)◀───────────────────open───────│  rollback  │───────┘            \n                            `─'                                └────────────┘                    \n                             ▲                                        │                          \n                             │                                    promoting                      \n                             │                                        │                          \n                             │                                        ▼                          \n                            .─.              .─.             ┌─────────────────┐                 \n                           (   )◀──errors───(   )◀──close────│confirm-promotion│                 \n                            `─'              `─'             └─────────────────┘                 \n                           check            pause                     │                          \n                          metrics                                   open                         \n                                                                      │                          \n                                                                      ▼                          \n                                                                     .─.                         \n                                                                    (███)                        \n                                                                     `─'                         \n                                                                   promote                       \n"

// createCliApp creates the CLI application using urfave/cli.
func createCliApp() *cli.Command {
	const OpenCommand = "open"
	const CloseCommand = "close"
	const StatusCommand = "status"
	var verboseCount int
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:     "cluster",
			Aliases:  []string{"c"},
			Usage:    "The alias of the Kubernetes cluster to use (as defined in your kubeconfig)",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "namespace",
			Aliases:  []string{"ns"},
			Usage:    "The namespace where the target service is located",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "deployment",
			Aliases:  []string{"d"},
			Usage:    "The name of the deployment to target",
			Required: false,
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Usage:   "Enable verbose logging",
			Aliases: []string{"v"},
			Config: cli.BoolConfig{
				Count: &verboseCount,
			},
			Action: func(ctx context.Context, c *cli.Command, b bool) error {
				return setLogLevel(verboseCount)
			},
		},
	}
	return &cli.Command{
		Name:  "canary-gate",
		Usage: "A CLI tool to interact with canary gate in Kubernetes",
		UsageText: `canary-gate [command] <gate-type> <global-options>

Example: canary-gate open confirm-rollout --cluster-alias my-cluster --namespace my-namespace --deployment my-deployment`,
		Description: "This tool allows you to open, close, and check the status of canary gate in a Kubernetes cluster.\n" +
			"It interacts with the canary-gate service running in the cluster to manage canary deployments.\n" +
			"Visits https://github.com/KongZ/canary-gate for more information.",
		UseShortOptionHandling: true,
		Commands: []*cli.Command{
			{
				Name:  OpenCommand,
				Usage: "Open a canary gate.",
				UsageText: `canary-gate open <gate-type> <global-options>

Example: canary-gate open confirm-rollout --cluster my-cluster --namespace my-namespace --deployment my-deployment`,
				Flags: flags,
				Commands: []*cli.Command{
					{
						Name:  string(service.HookConfirmRollout),
						Usage: "Enable the rollout of a new version.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:  string(service.HookPreRollout),
						Usage: "Allow the canary gate to adavance to pre-rollout state.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:  string(service.HookRollout),
						Usage: "Allow rollout to be continued.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:  string(service.HookConfirmTrafficIncrease),
						Usage: "Confirm the traffic increase after a rollout.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:  string(service.HookConfirmPromotion),
						Usage: "Allow to promote the canary version to production.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:  string(service.HookPostRollout),
						Usage: "Confirm the post-rollout tasks.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:  string(service.HookRollback),
						Usage: "Tell the canary gate to rollback the canary version. This gate can be opened during analysis or while waiting for a confirmation",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
				},
			},
			{
				Name:  CloseCommand,
				Usage: "Close a canary gate.",
				UsageText: `canary-gate close <gate-type> <global-options>

Example: canary-gate close confirm-rollout --cluster my-cluster --namespace my-namespace --deployment my-deployment`,
				Flags: flags,
				Commands: []*cli.Command{
					{
						Name:  string(service.HookConfirmRollout),
						Usage: "Halt the rollout of a new version until confirm-rollout gate is opened again.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:  string(service.HookPreRollout),
						Usage: "The canary advancement is paused if a pre-rollout gate is closed.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:  string(service.HookRollout),
						Usage: "Pause the rollout process and rollback if metrics check fails.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:  string(service.HookConfirmTrafficIncrease),
						Usage: "Pause the traffic increase after a rollout.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:  string(service.HookConfirmPromotion),
						Usage: "Halt the promotion of the canary version to production. While the promotion is paused, it will continue to run the metrics checks and rollout gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:  string(service.HookPostRollout),
						Usage: "Halt the post-rollout tasks",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:  string(service.HookRollback),
						Usage: "Close the rollout gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
				},
			},
			{
				Name:  StatusCommand,
				Usage: "Check status of a canary gate.",
				UsageText: `canary-gate status <gate-type> <global-options>

Example: canary-gate status confirm-rollout --cluster my-cluster --namespace my-namespace --deployment my-deployment`,
				Flags: flags,
				Commands: []*cli.Command{
					{
						Name:  string(service.HookConfirmRollout),
						Usage: "View the status of the confirm-rollout gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:  string(service.HookPreRollout),
						Usage: "View the status of the pre-rollout gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:  string(service.HookRollout),
						Usage: "View the status of the rollout gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:  string(service.HookConfirmTrafficIncrease),
						Usage: "View the status of the confirm-traffic-increase gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:  string(service.HookConfirmPromotion),
						Usage: "View the status of the confirm-promotion gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:  string(service.HookPostRollout),
						Usage: "View the status of the post-rollout gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:  string(service.HookRollback),
						Usage: "View the status of the rollback gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
				},
			},
			{
				Name:        "diagram",
				Usage:       "View the diagram of canary gate workflow",
				UsageText:   "View the diagram of canary gate workflow",
				Description: "Displays the diagram of the canary gate workflow, showing how each gate work with open/close command.\n\n" + diagram,
			},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "Enable verbose logging",
				Aliases: []string{"v"},
				Config: cli.BoolConfig{
					Count: &verboseCount,
				},
				Action: func(ctx context.Context, c *cli.Command, b bool) error {
					return setLogLevel(verboseCount)
				},
			},
		},
	}
}

func writePayload[I any](payload *I) []byte {
	r, err := json.Marshal(&payload)
	if err == nil {
		return r
	}
	return []byte{}
}

func setLogLevel(level int) error {
	if level == 2 {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if level >= 3 {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	return nil
}

// run contains the main logic of the command.
func run(ctx context.Context, cmd *cli.Command, gate string) error {
	canaryNs := "canary"
	canarySvc := "canary-gate"
	clusterAlias := cmd.String("cluster")
	if clusterAlias == "" {
		return fmt.Errorf("cluster name is required")
	}
	namespace := cmd.String("namespace")
	if namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	deployment := cmd.String("deployment")
	if deployment == "" {
		return fmt.Errorf("deployment name is required")
	}
	method := "POST"
	canaryPath := fmt.Sprintf("/%s", gate)
	canaryPort := 8080
	gateName := service.HookType(cmd.Name)
	payload := &handler.CanaryGatePayload{
		Type:      gateName,
		Name:      deployment,
		Namespace: namespace,
	}

	log.Info().
		Str("cluster", clusterAlias).
		Str("action", canaryPath).
		Str("gate", string(gateName)).
		Str("namespace", namespace).
		Str("deployment", deployment).
		Msg("Starting operation")

	// --- 1. Load Kubernetes Configuration ---
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: clusterAlias}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides)

	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubernetes config for cluster '%s': %w", clusterAlias, err)
	}
	log.Trace().Str("host", restConfig.Host).Msg("Kubernetes config loaded")

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// --- 2. Find a Pod for the Service ---
	service, err := clientset.CoreV1().Services(canaryNs).Get(ctx, canarySvc, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get service '%s' in namespace '%s': %w", canarySvc, canaryNs, err)
	}

	if len(service.Spec.Selector) == 0 {
		return fmt.Errorf("service '%s' has no selector, cannot find pods", canarySvc)
	}
	labelSelector := labels.SelectorFromSet(service.Spec.Selector).String()
	log.Trace().Str("selector", labelSelector).Msg("Found service selector")

	pods, err := clientset.CoreV1().Pods(canaryNs).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil || len(pods.Items) == 0 {
		return fmt.Errorf("failed to find any pods for service '%s' with selector '%s': %w", canarySvc, labelSelector, err)
	}

	var canaryPod *v1.Pod
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase == v1.PodRunning {
			canaryPod = pod
			break
		}
	}

	if canaryPod == nil {
		return fmt.Errorf("no running pods found for service '%s'", canarySvc)
	}
	log.Trace().Str("pod_name", canaryPod.Name).Msg("Found running pod backing the service")

	// --- 3. Make the HTTP Request via the API Server Proxy ---
	log.Trace().
		Str("method", method).
		Str("pod", canaryPod.Name).
		Int("port", canaryPort).
		Str("path", canaryPath).
		Msg("Proxying request to pod")

	// Manually construct the path to avoid incorrect URL escaping of the colon
	// by the default client-go URL builder.
	proxyPath := fmt.Sprintf(
		"/api/v1/namespaces/%s/pods/%s:%d/proxy%s",
		canaryNs,
		canaryPod.Name,
		canaryPort,
		canaryPath,
	)

	// Use AbsPath to set the full path for the request, bypassing the builder.
	req := clientset.CoreV1().RESTClient().Verb(method).AbsPath(proxyPath)

	// Attach the body if data is provided.
	req.Body(writePayload(payload))
	// Set a default content type. This can be customized further if needed.
	req.SetHeader("Content-Type", "application/json")

	// Execute the request and get the raw result.
	result := req.Do(ctx)
	if err := result.Error(); err != nil {
		return fmt.Errorf("request to pod proxy failed: %w", err)
	}

	// Get the raw response body.
	rawBody, err := result.Raw()
	if err != nil {
		return fmt.Errorf("failed to get raw response from proxy: %w", err)
	}

	// --- 4. Print the Response ---
	if status, err := readPayload(rawBody, handler.CanaryGateStatus{}); err != nil {
		return fmt.Errorf("failed to read response payload: %w", err)
	} else {
		log.Info().
			Str("status", status.Status).
			Str("name", status.Name).
			Str("namespace", status.Namespace).
			Str("gate", string(status.Type)).
			Msg("Canary Gate Status")
	}

	return nil
}

func readPayload[I any](payload []byte, i I) (*I, error) {
	err := json.Unmarshal(payload, &i)
	if err != nil {
		return &i, err
	}
	return &i, nil
}
