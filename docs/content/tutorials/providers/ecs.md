---
title: AWS ECS
weight: 7
---

This tutorial connects Sablier to Amazon ECS. You will select the ECS provider, give Sablier AWS credentials with the permissions it needs, register an ECS service with tags so Sablier can scale it, and confirm Sablier knows when the service is ready. The ECS provider communicates with the ECS API to move the desired count of a service between zero and its active replicas on demand.

## Select the ECS provider

Set the `provider.name` property to `ecs` and name the cluster that holds your services. See the [CLI reference](/reference/cli/) for every ECS flag.

{{< tabs >}}
{{< tab name="File (YAML)" >}}

```yaml
provider:
  name: ecs
  ecs:
    cluster: my-cluster
    region: eu-west-1
```

{{< /tab >}}
{{< tab name="CLI" >}}

```bash
sablier start \
  --provider.name=ecs \
  --provider.ecs.cluster=my-cluster \
  --provider.ecs.region=eu-west-1
```

{{< /tab >}}
{{< tab name="Environment Variable" >}}

```bash
SABLIER_PROVIDER_NAME=ecs
SABLIER_PROVIDER_ECS_CLUSTER=my-cluster
SABLIER_PROVIDER_ECS_REGION=eu-west-1
```

{{< /tab >}}
{{< /tabs >}}

The `cluster` accepts a short name or a full ARN and defaults to `default`. The `region` is optional. When it is empty, the AWS SDK resolves the region from `AWS_REGION`, the shared configuration profile, or the instance metadata. The optional `endpoint` overrides the ECS API URL, for example to target a local emulator.

## Give Sablier access to the ECS API

Sablier does not have credential settings. It uses the AWS SDK default credential chain, which reads credentials from these sources:

- The `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_SESSION_TOKEN` environment variables
- The shared configuration and credentials files (`~/.aws/config`, `~/.aws/credentials`) and the `AWS_PROFILE` variable
- A web identity token, for example IAM roles for service accounts on EKS
- The container credentials endpoint, which serves the task role of an ECS task and EKS Pod Identity
- The instance metadata service, which serves the instance role of an EC2 instance

Running Sablier as an ECS service in the same cluster with a task role is the recommended setup. The role needs this policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecs:DescribeClusters",
        "ecs:ListServices",
        "ecs:DescribeServices",
        "ecs:UpdateService"
      ],
      "Resource": "*"
    }
  ]
}
```

You can restrict `Resource` to the cluster ARN for `ecs:DescribeClusters` and `ecs:ListServices`, and to the service ARNs for `ecs:DescribeServices` and `ecs:UpdateService`.

## Register a service

For Sablier to work, it needs to know which ECS services to scale up and down. Register a service by opting in with **resource tags** on the service:

```bash
aws ecs tag-resource \
  --resource-arn arn:aws:ecs:eu-west-1:123456789012:service/my-cluster/whoami \
  --tags key=sablier.enable,value=true key=sablier.group,value=mygroup
```

In the AWS console, open the service, select the **Tags** tab and add the same keys.

With Terraform, set the tags on the service resource:

```hcl
resource "aws_ecs_service" "whoami" {
  name            = "whoami"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.whoami.arn
  desired_count   = 0

  tags = {
    "sablier.enable" = "true"
    "sablier.group"  = "mygroup"
  }
}
```

{{< callout type="warning" >}}
An ECS tag value cannot contain a comma. Separate list values with a **space** instead. For example, `sablier.group` with the value `team-a team-b` puts the service in both groups. This applies to `sablier.group`, `sablier.running-days` and `sablier.anti-affinity`.
{{< /callout >}}

Every other label from the [Label reference](/reference/labels/) works as a tag with the same key and value, for example `sablier.ready-after` with the value `30s` or `sablier.active.replicas` with the value `2`.

## Instance naming

Sablier uses the ECS **service name** as the instance name, for example `whoami`. The full service ARN is also accepted. Sablier manages one cluster per instance. To manage several clusters, run one Sablier per cluster.

## Confirm when the service is ready

Sablier reads the desired count, the running count and the primary deployment of the service.

| ECS state | Sablier status |
|---|---|
| Desired count is `0` | Not Ready (stopped) |
| Running count is below the desired count | Not Ready (starting) |
| Primary deployment rollout is `IN_PROGRESS` | Not Ready (starting) |
| Primary deployment rollout is `FAILED` | Unrecoverable |
| Running count reaches the desired count and the rollout is `COMPLETED` | Ready |

{{< callout type="info" >}}
ECS keeps the primary deployment `IN_PROGRESS` until the new tasks pass the container health check of the task definition. A service with a health check therefore becomes ready only when its containers are healthy. A service without a health check becomes ready as soon as its tasks are running. Add the `sablier.ready-after` tag to give such a service a settling delay.
{{< /callout >}}

When the deployment circuit breaker marks a rollout as `FAILED`, Sablier reports the instance as **Unrecoverable** with the rollout state reason from ECS. Fix the task definition or the service and start a new deployment to recover.

## Limitations

- Sablier changes the **desired count** only. The `sablier.idle.cpu`, `sablier.idle.memory`, `sablier.active.cpu` and `sablier.active.memory` tags are ignored with a warning, because on ECS the CPU and memory of a service belong to its task definition.
- Only services with the `REPLICA` scheduling strategy are managed. A `DAEMON` service has no desired count.
- ECS publishes service changes to EventBridge only. Sablier polls the cluster every 10 seconds to detect services that were started, stopped, tagged or removed outside of Sablier.
