[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Build](https://github.com/KongZ/canary-gate/actions/workflows/pull_request.yml/badge.svg)](https://github.com/KongZ/canary-gate/actions/workflows/pull_request.yml)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/canary-gate)](https://artifacthub.io/packages/search?repo=canary-gate)

![canary-gate logo](https://raw.githubusercontent.com/KongZ/canary-gate/main/docs/gopher-canary-gate.png "Canary Gate logo")

# Canary Gate

Canary Gate is a tool created for integration with [Flagger](https://docs.flagger.app/) that offers detailed management of Canary deployments, rather than depending entirely on automated metric analysis. Although it works alongside metrics analysis, the ultimate decision for each phase will be made using this tool.

# How it work

Flagger will provide the [webhook](https://docs.flagger.app/usage/webhooks) for each canary phase. The tool will communicate with Flagger and return either the advance or halt command to Flagger.

```text
      .─.        ┌───────────────┐                                 ┌──────────┐
     (   )──────▶│confirm-rollout│───────open─────────────────────▶│ rollout  │◀───────┐
      `─'        └───────────────┘                 ┌──close────────└──────────┘        │
     deploy              │                         │                     │             │
                       close                       ▼                     │             │
                         │                        .─.                  open            │
                         ▼                       (   )                   │             │
                        .─.                       `─'                    ▼             │
                       (   )                     pause                  .─.            │
                        `─'     ┌──────────────────────────────────────(   )           │
                       pause    │                                       `─'            │
                              errors                                   check          .─.
                                │                                     metrics        (   ) increase
                                │                                        │            `─'  traffic
                                │                                        │             ▲
                                │                                        ▼             │
                                │                               ┌────────────────┐     │
                                │            .─.                │confirm-traffic-│     │
                                │           (   )◀────close─────│    increase    │     │
                                │            `─'                └────────────────┘     │
                                │           pause                        │           close
                                │                                      open            │
                                │                                        │             │
                                ▼                                        ▼             │
                               .─.                                ┌────────────┐       │
                    rollback  ( █ )◀───────────────────open───────│  rollback  │───────┘
                               `─'                                └────────────┘
                                ▲                                        │
                                │                                    promoting
                                │                                        │
                                │                                        ▼
                               .─.              .─.             ┌─────────────────┐
                              (   )◀──errors───(   )◀──close────│confirm-promotion│
                               `─'              `─'             └─────────────────┘
                              check            pause                     │
                             metrics                                   open
                                                                         │
                                                                         ▼
                                                                        .─.
                                                                       ( █ )
                                                                        `─'
                                                                      promote
```

Each gate controls the flow of the Flagger Canary process.

1. When a new version is detected, it will check the <confirm-rollout> gate.
   - If the gate is open, it will proceed to the next stage.
   - If the gate is closed, it will halt the process and wait until the gate is opened.

2. Next, it will check the <pre-rollout> gate. This stage is not depicted in the diagram.
   - If the gate is open, it will proceed to the next stage.
   - If the gate is closed, it will halt the process and wait until the gate is opened.

3. Flagger will begin increasing traffic based on the configuration in CanaryGate. Before each traffic increase, it will check the <rollout> and <confirm-traffic-increase> gates.
   - If <rollout> is open, it will proceed to the next stage.
   - If <rollout> is closed, it will halt the process and continue monitoring metrics. If metrics indicate failure, it will initiate a rollback.
   - If <confirm-traffic-increase> is open, it will continue to increase traffic and proceed to the next stage.
   - If <confirm-traffic-increase> is closed, it will halt the process.

4. After increasing traffic until it reaches the maximum weight, it will check the <confirm-promotion> gate.
   - If the gate is open, it will proceed to promote to the new version.
   - If the gate is closed, it will halt the process and continue monitoring metrics. If metrics indicate failure, it will initiate a rollback.

5. Flagger will copy the canary deployment specification template over to the primary. After promotion is finalized, the <post-rollout> gate is checked. This stage is not depicted in the diagram.
   - If the gate is open, the process is completed.
   - If the gate is closed, the process is pending finalization.

6. The <rollback> gate is continuously monitored throughout the process.
   - If the gate is open, the rollback process is initiated.
   - If the gate is closed, the rollout process continues.

# Installation

## Prerequitsite

First, you will need to install Flagger on your Kubernetes cluster.

Install Flagger CRD

```bash
kubectl apply -f https://raw.githubusercontent.com/fluxcd/flagger/main/artifacts/flagger/crd.yaml
```

Flagger requires the installation of Istio or another service mesh to manage traffic effectively. It is recommended to set up Istio before continuing with the installation of Flagger in the next step. The metric server can be omitted at this stage. Please check the Flagger [metricc](https://docs.flagger.app/usage/metrics) documentation for a list of supported providers.

Deploy Flagger

```bash
helm repo add flagger https://flagger.app
helm upgrade -i flagger flagger/flagger \
--namespace=istio-system \
--set crd.create=false \
--set meshProvider=istio
```

See full installation detail from [https://docs.flagger.app/install/flagger-install-on-kubernetes](https://docs.flagger.app/install/flagger-install-on-kubernetes)

## Install Canary Gate

1. Run helm chart install

```bash
helm -n canary-gate install canary-gate oci://ghcr.io/kongz/helm-charts/canary-gate --version 0.1.1
```

If you encounter problems with the installed Custom Resource Definition (CRD) file, you may need to install the CRD prior to continuing with the Helm installation.

```bash
kubectl apply -f https://raw.githubusercontent.com/KongZ/canary-gate/main/docs/canarygate-crd.yaml
helm -n canary-gate install canary-gate oci://ghcr.io/kongz/helm-charts/canary-gate --set crd.create=false
```

## Configure Canary Gate

Assume that you already have an application deployment named demo within the `demo-ns` namespace.

```yaml
apiVersion: piggysec.com/v1alpha1
kind: CanaryGate
metadata:
  name: demo
spec:
  target:
    namespace: demo-ns
    name: demo
  flagger:
    targetRef:
      apiVersion: apps/v1
      kind: Deployment
      name: demo
    service:
      port: 8080
    skipAnalysis: false
    analysis:
      interval: 10s
      threshold: 2
      maxWeight: 40
      stepWeight: 10
      stepWeightPromotion: 100
```

Canary Gate contains `target` and `flagger`

```yaml
target:
  namespace: demo-ns
  name: demo
```

The target specifies the location of the `Canary` object. The CanaryGate will replicate all content under `flagger` to the Canary object upon execution. You can find the description and configuration instructions for Canary [https://docs.flagger.app/usage/how-it-works](https://docs.flagger.app/usage/how-it-works).

# Command-Line (CLI)

Use can the command-line tool to open/close gates.

## CLI Installation

<!-- TODO brew -->
<!-- TODO binary download -->
<!-- TODO code compile -->

# Sample Canary

You can find more sample from Flagger documents. There are few examples can be found in this repository.

## Pause the deployment

The CanaryGate configuration

```yaml
apiVersion: piggysec.com/v1alpha1
kind: CanaryGate
metadata:
  name: demo
spec:
  confirm-rollout: closed
  target:
    namespace: demo-ns
    name: demo
  flagger:
    targetRef:
      apiVersion: apps/v1
      kind: Deployment
      name: demo
    service:
      port: 8080
    skipAnalysis: false
    analysis:
      interval: 10s
      threshold: 2
      maxWeight: 40
      stepWeight: 10
      stepWeightPromotion: 100
```

This example sets `confirm-rollout: closed`. During the deployment, this configuration will pause the rollout process.
No new version will be deployed until `confirm-rollout` is set to `opened`.

## Hold the traffic at 80:20 until further notice

The CanaryGate configuration

```yaml
apiVersion: piggysec.com/v1alpha1
kind: CanaryGate
metadata:
  name: demo
spec:
  confirm-rollout: opened
  rollout: opened
  confirm-traffic-increase: opened
  confirm-promotion: closed
  target:
    namespace: demo-ns
    name: demo
  flagger:
    targetRef:
      apiVersion: apps/v1
      kind: Deployment
      name: demo
    service:
      port: 8080
    skipAnalysis: false
    analysis:
      interval: 10s
      threshold: 2
      maxWeight: 20
      stepWeight: 20
      stepWeightPromotion: 100
```

This example sets `confirm-promotion: closed`. During the deployment, the rollout process will continue until the new version captures 20% of the traffic from a total of 100%. At that stage, the promotion will be paused. While paused, the new version will still receive 20% of the traffic, while the old version will receive 80%. The process will stay paused until `confirm-traffic-increase` is set to `opened`.

## License

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

<http://www.apache.org/licenses/LICENSE-2.0/>

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
