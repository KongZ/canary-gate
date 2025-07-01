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

	// Create and run the CLI application.
	app := createCliApp()
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Err(err).Msg("Application failed")
	}
}

// createCliApp creates the CLI application using urfave/cli.
func createCliApp() *cli.Command {
	const OpenCommand = "open"
	const CloseCommand = "close"
	const StatusCommand = "status"
	return &cli.Command{
		Name:  "canary-gate",
		Usage: "A CLI tool to interact with canary gate tasks in Kubernetes",
		// UsageText: "canary-gate [command options] <cluster-alias> <namespace> <deployment-name>",
		Description: `canary-gate is a CLI tool to interact with canary gate tasks in Kubernetes.`,
		Commands: []*cli.Command{
			{
				Name:      OpenCommand,
				UsageText: "Open a canary gate task.",
				Commands: []*cli.Command{
					{
						Name:      string(service.HookConfirmRollout),
						UsageText: "Enable the rollout of a new version.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:      string(service.HookPreRollout),
						UsageText: "Allow the canary gate to adavance to pre-rollout state.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:      string(service.HookRollout),
						UsageText: "Allow rollout to be continued.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:      string(service.HookConfirmTrafficIncrease),
						UsageText: "Confirm the traffic increase after a rollout.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:      string(service.HookConfirmPromotion),
						UsageText: "Allow to promote the canary version to production.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:      string(service.HookPostRollout),
						UsageText: "Confirm the post-rollout tasks.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
					{
						Name:      string(service.HookRollback),
						UsageText: "Tell the canary gate to rollback the canary version. This gate can be opened during analysis or while waiting for a confirmation",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, OpenCommand)
						},
					},
				},
			},
			{
				Name:      CloseCommand,
				UsageText: "Close a canary gate task.",
				Commands: []*cli.Command{
					{
						Name:      string(service.HookConfirmRollout),
						UsageText: "Halt the rollout of a new version until confirm-rollout gate is opened again.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:      string(service.HookPreRollout),
						UsageText: "The canary advancement is paused if a pre-rollout gate is closed.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:      string(service.HookRollout),
						UsageText: "Pause the rollout process and rollback if metrics check fails.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:      string(service.HookConfirmTrafficIncrease),
						UsageText: "Pause the traffic increase after a rollout.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:      string(service.HookConfirmPromotion),
						UsageText: "Halt the promotion of the canary version to production. While the promotion is paused, it will continue to run the metrics checks and rollout gate.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:      string(service.HookPostRollout),
						UsageText: "Halt the post-rollout tasks",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:      string(service.HookRollback),
						UsageText: "Close the rollout gate.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
				},
			},
			{
				Name:      StatusCommand,
				UsageText: "View status of a canary gate task",
				Commands: []*cli.Command{
					{
						Name:      string(service.HookConfirmRollout),
						UsageText: "View the status of the confirm-rollout gate.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:      string(service.HookPreRollout),
						UsageText: "View the status of the pre-rollout gate.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:      string(service.HookRollout),
						UsageText: "View the status of the rollout gate.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:      string(service.HookConfirmTrafficIncrease),
						UsageText: "View the status of the confirm-traffic-increase gate.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:      string(service.HookConfirmPromotion),
						UsageText: "View the status of the confirm-promotion gate.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:      string(service.HookPostRollout),
						UsageText: "View the status of the post-rollout gate.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:      string(service.HookRollback),
						UsageText: "View the status of the rollback gate.",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
				},
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "cluster-alias",
				Aliases: []string{"c"},
				Usage:   "The alias of the Kubernetes cluster to use (as defined in your kubeconfig)",
			},
			&cli.StringFlag{
				Name:    "namespace",
				Aliases: []string{"ns"},
				Usage:   "The namespace where the target service is located",
			},
			&cli.StringFlag{
				Name:    "deployment",
				Aliases: []string{"d"},
				Usage:   "The name of the deployment to target",
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

// run contains the main logic of the command.
func run(ctx context.Context, cmd *cli.Command, action string) error {
	canaryNs := "canary"
	canarySvc := "canary-gate"
	clusterAlias := cmd.String("cluster-alias")
	namespace := cmd.String("namespace")
	deployment := cmd.String("deployment")
	method := "POST"
	canaryPath := fmt.Sprintf("/%s", action)
	canaryPort := 8080
	hook := service.HookType(cmd.Name)
	payload := &handler.CanaryGatePayload{
		Type:      hook,
		Name:      deployment,
		Namespace: namespace,
	}

	log.Info().
		Str("cluster", clusterAlias).
		Str("action", canaryPath).
		Str("hook", string(hook)).
		Str("namespace", namespace).
		Str("deployment", deployment).
		Msg("Starting operation using API Proxy")

	// --- 1. Load Kubernetes Configuration ---
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: clusterAlias}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides)

	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubernetes config for cluster '%s': %w", clusterAlias, err)
	}
	log.Info().Str("host", restConfig.Host).Msg("Kubernetes config loaded")

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
	log.Info().Str("selector", labelSelector).Msg("Found service selector")

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
	log.Info().Str("pod_name", canaryPod.Name).Msg("Found running pod backing the service")

	// --- 3. Make the HTTP Request via the API Server Proxy ---
	log.Info().
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
	log.Info().Str("path", proxyPath).Msg("Constructed proxy path")

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
	// Note: The proxy endpoint streams the raw response. It does not provide a structured
	// http.Response object with headers and status code through the client.
	// We print the raw body directly.
	log.Info().Msg("--- Proxied Response Body ---")
	fmt.Println(string(rawBody))

	return nil
}
