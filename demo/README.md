# Demo
This folder contains sample manifests and configurations to demonstrate the usage and integration of canary-gate with Flagger.

## Files

`deployment.yaml`

A sample Kubernetes deployment manifest for the test application. This file is used to validate and demonstrate the canary-gate workflow in a real deployment scenario.

`canarygate-pause-new-version.yaml`

Demonstrates how to halt all new version deployments until the confirm-rollout gate is opened. This manifest is useful for enforcing manual approval before a new version is rolled out in the canary process.

Sample command to open confirm-rollout gate on cluster named my-cluster and configuration named demo

```sh
canary-gate open confirm-rollout -c my-cluster -d demo
```


`canarygate-pause-promotion.yaml`

Shows how to pause the promotion of a new version and maintain the traffic ratio between the new and old versions at 20:80. Promotion will only proceed when the confirm-promotion gate is opened, allowing for controlled traffic shifting and manual intervention.

Sample command to open confirm-promotion gate on cluster named my-cluster and configuration named demo

```sh
canary-gate open confirm-promotion -c my-cluster -d demo
```

`metrics.yaml`

Provides sample Flagger metric templates for monitoring. It includes configurations for Datadog and Prometheus providers, enabling error rate and CPU usage monitoring for canary analysis.
