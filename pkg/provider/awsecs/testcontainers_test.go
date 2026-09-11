package awsecs_test

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"gotest.tools/v3/assert"
)

// localstackImage uses the tag without zero padding. The localstack module
// rejects 2026.08.2 because it is not valid semver (isMinimumVersion in modules/localstack).
const localstackImage = "localstack/localstack:2026.8.2"

const mimicImage = "sablierapp/mimic:v0.3.3"

// setupLocalStack starts LocalStack and returns an ECS client for it. ECS is not
// in the free LocalStack plan, so the test skips without LOCALSTACK_AUTH_TOKEN.
func setupLocalStack(t *testing.T) *ecs.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	token := os.Getenv("LOCALSTACK_AUTH_TOKEN")
	if token == "" {
		t.Skip("skipping ECS integration test: LOCALSTACK_AUTH_TOKEN is not set")
	}

	ctx := t.Context()
	c, err := localstack.Run(ctx, localstackImage, testcontainers.WithEnv(map[string]string{
		"LOCALSTACK_AUTH_TOKEN": token,
	}))
	testcontainers.CleanupContainer(t, c)
	assert.NilError(t, err, "cannot start LocalStack")

	endpoint, err := c.PortEndpoint(ctx, "4566/tcp", "http")
	assert.NilError(t, err)
	return newECSClient(endpoint)
}

// newECSClient returns an ECS client for an emulator endpoint. It does not read
// the AWS configuration of the machine, so a local profile cannot change the test.
func newECSClient(endpoint string) *ecs.Client {
	return ecs.New(ecs.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test", Source: "sablier-integration-test"}, nil
		}),
	})
}
