---
title: AWS ECS
weight: 7
since: next
aliases:
  - /tutorials/providers/ecs/
---

## Introduction

The AWS ECS provider sets the [desired count](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#sd-desiredcount) of [ECS services](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_services.html) to zero when their session expires, and back up when a request needs them. ECS then stops or starts the tasks. Sablier manages the services of one [cluster](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/clusters.html) and calls the ECS API with the [credentials of the AWS SDK](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html#specifying-credentials).

## Install Sablier

Run Sablier as an [ECS service](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_services.html) in the [cluster](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/clusters.html) it manages. Create the cluster with a default [Service Connect](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html) namespace, then deploy the [CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/Welcome.html) template:

```bash
aws ecs create-cluster --cluster-name sablier --service-connect-defaults namespace=sablier
aws cloudformation deploy --stack-name sablier --template-file sablier.yaml \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides Subnets=subnet-aaaa,subnet-bbbb SecurityGroup=sg-cccc
```

The template runs Sablier on [Fargate](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/AWS_Fargate.html). Its [task definition](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definitions.html) selects the ECS provider and the cluster:

{{< example file="ecs/sablier.yaml" region="config" >}}

Sablier gets its credentials from the [task role](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html). The role's [IAM policy](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html) allows the four [ECS API actions](https://docs.aws.amazon.com/service-authorization/latest/reference/list_ecs.html) Sablier calls:

{{< example file="ecs/sablier.yaml" region="iam" >}}

- **`ecs:ListServices` needs `"Resource": "*"`.** The action has no resource type, so scope it to the cluster with the `ecs:cluster` condition key.
- **`ecs:UpdateService` can require the `sablier.enable` tag.** The [`aws:ResourceTag`](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_condition-keys.html#condition-keys-resourcetag) condition stops Sablier from scaling services that did not opt in.

Reverse proxy plugins reach Sablier at `http://sablier:10000`. The service registers that name in the [Service Connect](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html) namespace, so run your reverse proxy as an ECS service in the same namespace:

{{< example file="ecs/sablier.yaml" region="service-connect" >}}

{{< details title="Run Sablier outside ECS" closed="true" >}}
Sablier reads [credentials](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html#specifying-credentials) the way every AWS SDK does, and uses the first source it finds:

- The `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables.
- A web identity token, such as EKS IAM roles for service accounts.
- The shared `config` and `credentials` files, with `AWS_PROFILE`.
- The container credentials of an ECS task role or EKS Pod Identity.
- The instance role of an EC2 instance.

Grant those credentials the policy above. Set the [region](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html#specifying-the-aws-region) with `--provider.ecs.region` or `AWS_REGION`. Sablier does not read it from the EC2 instance metadata.
{{< /details >}}

## Usage example

Opt an [ECS service](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_services.html) in with the `sablier.enable` [tag](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-using-tags.html), and give it a group:

{{< example file="ecs/whoami.yaml" >}}

```bash
aws cloudformation deploy --stack-name whoami --template-file whoami.yaml \
  --parameter-overrides Subnets=subnet-aaaa,subnet-bbbb SecurityGroup=sg-cccc
```

Then configure your [reverse proxy](/reverse-proxies/) to call Sablier for the `whoami` group in front of `http://whoami`. On the first request, Sablier sets the [desired count](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#sd-desiredcount) to `1` and holds the request, or shows a waiting page, until the task is running. When the session expires, Sablier sets it back to `0`.

{{< callout type="info" >}}
Sablier sets the desired count of opted-in services to zero when it starts. A service you deploy afterwards keeps running until its first session expires, unless you [stop externally-started instances](/features/stop-external-starts/).
{{< /callout >}}

## Provider reference

### Configuration

| Option | Default | Description |
|---|---|---|
| [`--provider.ecs.cluster`](/reference/cli/#opt-provider-ecs-cluster) | `default` | The name or ARN of the [cluster](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/clusters.html) that holds the services. Sablier fails to start if the cluster is missing or not `ACTIVE`. |
| [`--provider.ecs.region`](/reference/cli/#opt-provider-ecs-region) | | The AWS [region](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html#specifying-the-aws-region) of the cluster. When empty, Sablier reads `AWS_REGION` or the shared config profile. |
| [`--provider.ecs.endpoint`](/reference/cli/#opt-provider-ecs-endpoint) | | Replaces the [ECS API endpoint](https://docs.aws.amazon.com/general/latest/gr/ecs-service.html) URL, for example to target an emulator such as LocalStack. |

Every `--provider.*` option that is not provider-specific also applies; see the [CLI reference](/reference/cli/).

### Workloads

Sablier scales [ECS services](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_services.html) that use the `REPLICA` [scheduling strategy](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#sd-schedulingstrategy). It calls [`UpdateService`](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_UpdateService.html) to change the [desired count](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#sd-desiredcount):

- **Stop** sets the desired count to `0`, or to [`idle.replicas`](#scale-instead-of-stop) when set.
- **Start** sets it to [`active.replicas`](#scale-instead-of-stop) when set, or `1`.

ECS then starts or stops tasks from the service's [task definition](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definitions.html). Sablier never changes the task definition.

### Instance names

A reverse proxy can target one service by name instead of a group. The `whoami` service of the usage example is named:

```text
whoami
```

The instance name is the [service name](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html), without the cluster. Service names are only unique within a cluster, so one Sablier manages one cluster. A request can also name the service by its [ARN](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-account-settings.html#ecs-resource-ids): it resolves to the service name and shares its session.

Prefer groups: a name ties your proxy configuration to the service.

### Labels

On ECS, Sablier settings are [resource tags](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-using-tags.html) on the service. The keys and values are those of the [label reference](/reference/labels/):

{{< example file="ecs/group.yaml" region="sablier" >}}

- **Tag the service.** Sablier reads the tags of the [service](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_services.html) only, not those of its task definition or tasks.
- **Separate list values with spaces.** [Tag values can't contain commas](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-using-tags.html#tag-restrictions), so lists of groups and weekdays are space-separated.
- **Keys and values are case-sensitive.** `sablier.enable` must be exactly `true`.

To tag a service you did not create with CloudFormation, use the AWS CLI:

```bash
aws ecs tag-resource --resource-arn arn:aws:ecs:eu-west-1:123456789012:service/sablier/whoami \
  --tags key=sablier.enable,value=true key=sablier.group,value=whoami
```

### Workload settings

Each excerpt below is the service of a complete template in [`examples/ecs`](https://github.com/sablierapp/sablier/tree/main/examples/ecs).

#### Enable {since="next"}

The `sablier.enable` [tag](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-using-tags.html) set to `true` opts the service in. It is required, and any other value is ignored. See the [usage example](#usage-example).

#### Group {since="next"}

`sablier.group` puts the service in one or more groups. A session for any of them starts it. Without it, the service is in the `default` group.

Separate the groups with spaces, because a [tag value](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-using-tags.html#tag-restrictions) can't contain a comma:

{{< example file="ecs/group.yaml" region="sablier" >}}

[Learn more about groups](/features/groups/).

#### Ready after {since="next"}

`sablier.ready-after` keeps reporting the service as not ready for a settling delay after its tasks first reach the [`RUNNING` state](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-lifecycle-explanation.html). Use it when your application needs time to serve traffic after its task starts.

{{< example file="ecs/ready-after.yaml" region="sablier" >}}

[Learn more about delaying readiness](/features/ready-after/).

#### Ready on start {since="next"}

`sablier.ready-on-start` set to `true` reports the service ready as soon as Sablier raises its [desired count](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#sd-desiredcount), without waiting for its tasks.

{{< example file="ecs/ready-on-start.yaml" region="sablier" >}}

[Learn more about marking ready on start](/features/ready-on-start/).

#### Running hours {since="next"}

- `sablier.running-hours` keeps the service running during a daily window.
- `sablier.running-days` limits that window to some weekdays. Separate the days with spaces.

{{< example file="ecs/running-hours.yaml" region="sablier" >}}

The window uses Sablier's local time. Set the time zone with the `TZ` [environment variable](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#container_definition_environment) of Sablier's container:

{{< example file="ecs/sablier.yaml" region="timezone" >}}

[Learn more about running hours](/features/running-hours/).

#### Anti-affinity {since="next"}

`sablier.anti-affinity` lists groups the service backs off from: Sablier keeps it idle while any of them has an active session. Separate the groups with spaces.

{{< example file="ecs/anti-affinity.yaml" region="sablier" >}}

[Learn more about anti-affinity](/features/anti-affinity/).

#### Scale instead of stop {since="next"}

Keep the service running when idle instead of scaling it to zero:

- `sablier.idle.replicas` set to `1` or more keeps that many tasks when idle.
- `sablier.active.replicas` sets the [desired count](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#sd-desiredcount) on the next session.

{{< example file="ecs/scale-mode.yaml" region="sablier" >}}

On ECS, only the task count changes. The CPU and memory of a task belong to its [task definition](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definitions.html), so Sablier ignores the CPU and memory tags and logs a warning.

[Learn more about scaling instead of stopping](/features/scale-mode/).

### Readiness

Sablier reads the [service](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html) and its `PRIMARY` [deployment](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Deployment.html), and reports it:

- **stopped** when its desired count is `0`, even while tasks are still stopping.
- **error** when the rollout state of the primary deployment is `FAILED`. Sablier shows the reason ECS gives.
- **starting** while fewer tasks are running than desired, or while the primary deployment is `IN_PROGRESS`.
- **ready** otherwise.

ECS counts a task as running once it reaches the [`RUNNING` state](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-lifecycle-explanation.html). If your application needs more time after that, add [`sablier.ready-after`](#ready-after).

A rollout fails when the [deployment circuit breaker](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-circuit-breaker.html) is on and the service can't reach a steady state. Sablier reports the error until a new deployment replaces the failed one.

### Limitations

- One Sablier manages one [cluster](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/clusters.html). Run one Sablier per cluster.
- Sablier ignores services that use the `DAEMON` [scheduling strategy](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#sd-schedulingstrategy): they have no desired count to scale.
- [CPU](/features/cpu-limits/), [memory](/features/memory-limits/) and [block I/O](/features/disk-io-limits/) throttling are not possible on ECS: they belong to the [task definition](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definitions.html).
- ECS publishes service changes to [EventBridge](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html) only. Sablier polls the cluster every 10 seconds to detect services started, stopped, tagged or removed outside Sablier.

## Examples

- [`examples/ecs`](https://github.com/sablierapp/sablier/tree/main/examples/ecs): every template on this page, with instructions to deploy them.
