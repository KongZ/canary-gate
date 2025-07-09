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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// main is the entry point of the application.
func main() {
	// Setup structured, human-friendly logging.
	writer := zerolog.ConsoleWriter{
		Out: os.Stdout,
		FormatFieldValue: func(i any) string {
			if s, ok := i.(string); ok {
				switch s {
				case "opened":
					return fmt.Sprintf("\x1b[32m ✅ %s\x1b[0m", s) // Example: Green for "opened"
				case "closed":
					return fmt.Sprintf("\x1b[31m 🔒 %s\x1b[0m", s) // Example: Red for "closed"
				}
			}
			return fmt.Sprintf("%v", i)
		},
		TimeFormat: time.RFC3339,
		FormatTimestamp: func(i any) string {
			if t, ok := i.(time.Time); ok {
				return t.Format(time.RFC3339)
			}
			return ""
		},
	}
	log.Logger = zerolog.New(writer).With().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Create and run the CLI application.
	app := createCliApp()
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Err(err).Msg("Application failed")
	}
}

// ServiceFQDN holds the parsed components of a Kubernetes service FQDN.
type ServiceFQDN struct {
	ServiceName string
	Namespace   string
	Port        string
}

// diagram is a string representation of the canary gate workflow diagram.
const diagram = "   .─.        ┌───────────────┐                                 ┌──────────┐                     \n  (   )──────▶│confirm-rollout│───────open─────────────────────▶│ rollout  │◀───────┐            \n   `─'        └───────────────┘                 ┌──close────────└──────────┘        │            \n  deploy              │                         │                     │             │            \n                    close                       ▼                     │             │            \n                      │                        .─.                  open            │            \n                      ▼                       (   )                   │             │            \n                     .─.                       `─'                    ▼             │            \n                    (   )                     pause                  .─.            │            \n                     `─'     ┌──────────────────────────────────────(   )           │            \n                    pause    │                                       `─'            │            \n                           errors                                   check          .─.           \n                             │                                     metrics        (   ) increase \n                             │                                        │            `─'  traffic  \n                             │                                        │             ▲            \n                             │                                        ▼             │            \n                             │                               ┌────────────────┐     │            \n                             │            .─.                │confirm-traffic-│     │            \n                             │           (   )◀────close─────│    increase    │     │            \n                             │            `─'                └────────────────┘     │            \n                             │           pause                        │           close          \n                             │                                      open            │            \n                             │                                        │             │            \n                             ▼                                        ▼             │            \n                            .─.                                ┌────────────┐       │            \n                 rollback  (███)◀───────────────────open───────│  rollback  │───────┘            \n                            `─'                                └────────────┘                    \n                             ▲                                        │                          \n                             │                                    promoting                      \n                             │                                        │                          \n                             │                                        ▼                          \n                            .─.              .─.             ┌─────────────────┐                 \n                           (   )◀──errors───(   )◀──close────│confirm-promotion│                 \n                            `─'              `─'             └─────────────────┘                 \n                           check            pause                     │                          \n                          metrics                                   open                         \n                                                                      │                          \n                                                                      ▼                          \n                                                                     .─.                         \n                                                                    (███)                        \n                                                                     `─'                         \n                                                                   promote                       \n"

const serviceLabel = "app=canary-gate"
const servicePortName = "http"
const defaultNamespace = "canary-gate"

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
			Aliases:  []string{"n"},
			Usage:    "The namespace where the CanaryGate resources is located",
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
		Usage: "A CLI tool to interact with canary gate in the Flagger",
		UsageText: `canary-gate [command] <gate-name> <global-options>

Example: canary-gate open confirm-rollout --cluster-alias my-cluster --namespace gate-namespace  --deployment my-deployment

The example command will look for a canary gate configuration named 'my-deployment' under 'gate-namespace' namespace in the 'my-cluster' cluster and open the 'confirm-rollout' gate for that deployment.

apiVersion: piggysec.com/v1alpha1
kind: CanaryGate
metadata:
  name: demo
	namespace: gate-namespace
spec:
  confirm-rollout: opened
  target:
    namespace: demo
    name: demo
  flagger:
    targetRef:
      apiVersion: apps/v1
      kind: Deployment
      name: demo
    skipAnalysis: false
    analysis:
      interval: 10s
      threshold: 2
      maxWeight: 50
      stepWeight: 10
`,
		Description: "This tool allows you to open, close, and check the status of canary gate in the Flagger.\n" +
			"It interacts with the canary-gate service running in the cluster to manage canary deployments.\n" +
			"Visits https://github.com/KongZ/canary-gate for more information.",
		UseShortOptionHandling: true,
		Commands: []*cli.Command{
			{
				Name:  OpenCommand,
				Usage: "Open a canary gate.",
				UsageText: `canary-gate open <gate-name> <global-options>

Example: canary-gate open confirm-rollout --cluster my-cluster --namespace gate-namespace --deployment my-deployment`,
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
						Name:   string(service.HookPreRollout),
						Usage:  "Allow the canary gate to adavance from pre-rollout state.",
						Hidden: true, // Hide this gate. It it not useful.
						Flags:  flags,
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
						Name:   string(service.HookPostRollout),
						Usage:  "Confirm the post-rollout tasks.",
						Hidden: true, // Hide this gate. It it not useful.
						Flags:  flags,
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
				UsageText: `canary-gate close <gate-name> <global-options>

Example: canary-gate close confirm-rollout --cluster my-cluster --namespace gate-namespace --deployment my-deployment`,
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
						Name:   string(service.HookPreRollout),
						Usage:  "The canary advancement is paused if a pre-rollout gate is closed.",
						Hidden: true, // Hide this gate. It it not useful.
						Flags:  flags,
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
						Name:   string(service.HookPostRollout),
						Usage:  "Halt the post-rollout tasks",
						Hidden: true, // Hide this gate. It it not useful.
						Flags:  flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, CloseCommand)
						},
					},
					{
						Name:  string(service.HookRollback),
						Usage: "Close the rollback gate. The rollback is still allowed if metrics check fails.",
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
				UsageText: `canary-gate status <gate-name> <global-options>

Example: canary-gate status confirm-rollout --cluster my-cluster --namespace gate-namespace --deployment my-deployment`,
				Flags: flags,
				Commands: []*cli.Command{
					{
						Name:  "all",
						Usage: "View status of all gates.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:  string(service.HookConfirmRollout),
						Usage: "View the status of the confirm-rollout gate.",
						Flags: flags,
						Action: func(ctx context.Context, cmd *cli.Command) error {
							return run(ctx, cmd, StatusCommand)
						},
					},
					{
						Name:   string(service.HookPreRollout),
						Usage:  "View the status of the pre-rollout gate.",
						Flags:  flags,
						Hidden: true, // Hide this gate. It it not useful.
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
						Name:   string(service.HookPostRollout),
						Usage:  "View the status of the post-rollout gate.",
						Hidden: true, // Hide this gate. It it not useful.
						Flags:  flags,
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
				Name:      "explain",
				Usage:     "View the diagram and explain how of canary gate work",
				UsageText: "View the diagram and explain how of canary gate work",
				Description: "Displays the diagram of the canary gate workflow, showing how each gate work with open/close command.\n\n" +
					diagram + `
Gated canary promotion stages:
* Scan for canary deployments
* Check confirm-rollout gate
  * halt advancement is the gate is closed
* Check pre-rollout gate (This stage is hidden on the diagram)
  * halt advancement is the gate is closed
* Increase canary traffic (step weight or iteration)
	* Check rollout gate
		* If gate is open, continue checking metrics
	* Check canary metrics (If enabled)
		* Halt advancement if any metric is under the specified threshold
		* Scale new deployment to zero if the number of failed checks reached the threshold
	* Check confirm-traffic-increase gate
		* If gate is open, increase canary traffic weight (step weight) till it reaches (max weight)
* Check confirm-promotion gate
  * Halt advancement if gate is closed
	* If gate is open, copy canary deployment spec template over primary
* Check post-rollout gate when canary has been promoted or rolled back
	* Halt advancement if gate is closed
* If rollout gate is opened, rollback the canary deployment anytime during the canary promotion process.

Example of canarygate CRD file:

apiVersion: piggysec.com/v1alpha1
kind: CanaryGate
metadata:
  name: demo
	namespace: gate-namespace
spec:
  confirm-rollout: opened
  target:
    namespace: demons
    name: demo
  flagger:
    targetRef:
      apiVersion: apps/v1
      kind: Deployment
      name: demo
    skipAnalysis: false
    analysis:
      interval: 10s
      threshold: 2
      maxWeight: 50
      stepWeight: 10

The configuration above will create the Flagger Canary on the 'demons' namespace with the name 'demo' and it will copy all configuration under 'flagger' field to Flagger Canary. After that the Flagger Canary will be monitored by the canary-gate controller and if CanaryGate is modified, the controller will update the Flagger Canary accordingly.
`,
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

// setLogLevel sets the global log level based on the verbosity count.
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
	clusterAlias := cmd.String("cluster")
	if clusterAlias == "" {
		return fmt.Errorf("cluster name is required")
	}
	deployment := cmd.String("deployment")
	if deployment == "" {
		return fmt.Errorf("deployment name is required")
	}
	namespace := cmd.String("namespace")
	if namespace == "" {
		namespace = defaultNamespace
		log.Debug().Msgf("Namespace is not specified, using default namespace '%s'", defaultNamespace)
	}
	method := "POST"
	canaryPath := fmt.Sprintf("/%s", gate)
	payload := &handler.CanaryGatePayload{
		Type:      service.HookType(cmd.Name),
		Name:      deployment,
		Namespace: namespace,
	}

	log.Debug().
		Str("cluster", clusterAlias).
		Str("action", canaryPath).
		Str("gate", string(payload.Type)).
		Str("namespace", namespace).
		Str("deployment", deployment).
		Msg("Starting operation")

	//  Load Kubernetes Configuration
	clientset, err := loadKubernetesConfig(clusterAlias)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	// Find service by label
	service, err := findServiceByLabel(clientset, namespace, serviceLabel)
	if err != nil {
		return fmt.Errorf("failed to find service with label '%s' in namespace '%s'", serviceLabel, namespace)
	}

	// Find a Pod for the Service
	canaryPod, err := findRunningPod(ctx, clientset, namespace, service.Name)
	if err != nil {
		return fmt.Errorf("%w for service '%s'", err, service.Name)
	}

	// Find open port
	podPort, err := findPodPortFromServicePort(canaryPod, service, servicePortName)
	if err != nil {
		return fmt.Errorf("failed to find port '%s' in service '%s': %w", servicePortName, service.Name, err)
	}

	log.Trace().Str("pod_name", canaryPod.Name).Msg("Found running pod backing the service")

	// Make the HTTP Request via the API Server Proxy
	log.Trace().
		Str("method", method).
		Str("pod", canaryPod.Name).
		Str("service namespace", namespace).
		Str("service name", service.Name).
		Int("service port", podPort).
		Str("path", canaryPath).
		Msg("Proxying request to pod")

	// Manually construct the path to avoid incorrect URL escaping of the colon by the default client-go URL builder.
	proxyPath := fmt.Sprintf(
		"/api/v1/namespaces/%s/pods/%s:%d/proxy%s",
		namespace,
		canaryPod.Name,
		podPort,
		canaryPath,
	)

	// Use AbsPath to set the full path for the request, bypassing the builder.
	req := clientset.CoreV1().RESTClient().Verb(method).AbsPath(proxyPath)
	req.Body(writePayload(payload))
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

	// Print the Response
	if statusMap, err := readPayload(rawBody, map[string][]handler.CanaryGateStatus{}); err != nil {
		return fmt.Errorf("failed to read response payload: %w", err)
	} else {
		for k, v := range *statusMap {
			pad := "%-25s"
			if len(v) == 1 {
				pad = "%s"
			}
			for _, s := range v {
				log.Info().
					Str("gate", fmt.Sprintf(pad, string(s.Type))).
					Str("status", string(s.Status)).
					Msgf("Canary Gate Status for [%s]", k)
			}
		}
	}
	return nil
}

// loadKubernetesConfig loads the Kubernetes configuration for the specified cluster alias.
func loadKubernetesConfig(clusterAlias string) (*kubernetes.Clientset, error) {
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: clusterAlias}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides)

	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubernetes config for cluster '%s': %w", clusterAlias, err)
	}
	log.Trace().Str("host", restConfig.Host).Msg("Kubernetes config loaded")

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	return clientset, nil
}

// findServiceByLabel finds the first service that matches the given label selector.
func findServiceByLabel(clientset *kubernetes.Clientset, namespace, labelSelector string) (*corev1.Service, error) {
	services, err := clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	if len(services.Items) == 0 {
		return nil, fmt.Errorf("no services found with label selector '%s'", labelSelector)
	}
	// Return the first service found
	return &services.Items[0], nil
}

// findRunningPod locates a running pod associated with a given Kubernetes service.
// It first retrieves the service definition to find its label selector. Then, it
// lists all pods matching that selector within the specified namespace. It iterates
// through the resulting pods and returns the first one that is in the 'Running' state.
//
// An error is returned if the service cannot be found, if the service has no
// selector, if no pods match the selector, or if none of the matching pods are
// currently running.
func findRunningPod(ctx context.Context, clientset *kubernetes.Clientset, namespace string, svc string) (*corev1.Pod, error) {
	service, err := clientset.CoreV1().Services(namespace).Get(ctx, svc, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service '%s' in namespace '%s': %w", svc, namespace, err)
	}

	if len(service.Spec.Selector) == 0 {
		return nil, fmt.Errorf("service '%s' has no selector, cannot find pods", svc)
	}
	labelSelector := labels.SelectorFromSet(service.Spec.Selector).String()
	log.Trace().Str("selector", labelSelector).Msg("Found service selector")

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil || len(pods.Items) == 0 {
		return nil, fmt.Errorf("failed to find any pods for service '%s' with selector '%s': %w", svc, labelSelector, err)
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase == corev1.PodRunning {
			return pod, nil
		}
	}
	return nil, fmt.Errorf("no running pods found")
}

// findPodPortFromServicePort resolves a service port to a numeric pod container port.
func findPodPortFromServicePort(pod *corev1.Pod, service *corev1.Service, servicePortName string) (int, error) {
	var servicePort *corev1.ServicePort
	// Find the service port with the matching name
	for _, p := range service.Spec.Ports {
		if p.Name == servicePortName {
			servicePort = &p
			break
		}
	}
	if servicePort == nil {
		return 0, fmt.Errorf("service '%s' does not have a port named '%s'", service.Name, servicePortName)
	}

	// Check if the targetPort is a number or a name
	targetPort := servicePort.TargetPort
	if targetPort.Type == intstr.Int {
		// Target port is a number, return it directly
		return targetPort.IntValue(), nil
	}

	// Target port is a name, look it up in the pod's container ports
	if targetPort.Type == intstr.String {
		namedPort := targetPort.String()
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == namedPort {
					return int(port.ContainerPort), nil
				}
			}
		}
	}

	return 0, fmt.Errorf("could not find matching named port '%s' in pod '%s'", targetPort.String(), pod.Name)
}

func readPayload[I any](payload []byte, i I) (*I, error) {
	log.Trace().Bytes("raw", payload).Msg("Reading payload")
	err := json.Unmarshal(payload, &i)
	if err != nil {
		return &i, err
	}
	return &i, nil
}

func writePayload[I any](payload *I) []byte {
	r, err := json.Marshal(&payload)
	if err == nil {
		return r
	}
	return []byte{}
}
