# AWS ECS Examples

Runnable [CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/Welcome.html)
templates for the [AWS ECS provider](https://sablierapp.dev/providers/ecs/).
The documentation includes these files directly, so what you read is what you
deploy. The workloads use [sablierapp/mimic](https://github.com/sablierapp/mimic),
a configurable web-server built for testing purposes.

## Files

| File | Shows |
|---|---|
| `sablier.yaml` | Sablier as a Fargate service: task role, task definition and service |
| `whoami.yaml` | A service with no running task, the starting point for every other example |
| `group.yaml` | One service in two groups (space separated, because of the tag syntax) |
| `ready-after.yaml` | A settling delay after the tasks are running |
| `ready-on-start.yaml` | Ready as soon as the desired count is raised |
| `running-hours.yaml` | Kept warm on weekdays during office hours |
| `anti-affinity.yaml` | Forced idle while the `streaming` or `gaming` group is active |
| `scale-mode.yaml` | Kept at one task when idle instead of zero |

Lines between `# region <name>` and `# endregion` are the excerpts the
documentation shows; CloudFormation ignores the markers.

## Run it

You need the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html),
a VPC with subnets that can reach the internet, and a security group that allows
TCP `80` and `10000` from within itself.

Create the cluster with a default [Service Connect](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html)
namespace, then deploy Sablier and `whoami`:

```bash
aws ecs create-cluster --cluster-name sablier --service-connect-defaults namespace=sablier

NETWORK="Subnets=subnet-aaaa,subnet-bbbb SecurityGroup=sg-cccc"

aws cloudformation deploy --stack-name sablier --template-file sablier.yaml \
  --capabilities CAPABILITY_IAM --parameter-overrides $NETWORK
aws cloudformation deploy --stack-name whoami --template-file whoami.yaml \
  --parameter-overrides $NETWORK
```

Request the `whoami` group through the Sablier API, as a reverse proxy plugin
would, from a task in the `sablier` namespace (for example with
[ECS Exec](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html)
into your reverse proxy). The request opens a one-minute session:

```bash
curl "http://sablier:10000/api/strategies/blocking?group=whoami&session_duration=1m"
```

Sablier raises the desired count of `whoami` to `1` and answers once the task is
running. One minute after the last request, it sets the desired count back to
`0`:

```bash
aws ecs describe-services --cluster sablier --services whoami \
  --query 'services[0].[desiredCount,runningCount]'
```

Deploy any other file the same way, with its own stack name, to try that setting.
