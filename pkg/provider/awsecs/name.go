package awsecs

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

// InstanceName implements sablier.InstanceNameResolver: a service ARN, in the
// current (service/<cluster>/<name>) or old (service/<name>) format, resolves
// to the service name that InstanceInspect reports.
func (p *Provider) InstanceName(_ context.Context, name string) string {
	a, err := arn.Parse(name)
	if err != nil || a.Service != "ecs" {
		return name
	}
	resource, ok := strings.CutPrefix(a.Resource, "service/")
	if !ok {
		return name
	}
	return resource[strings.LastIndex(resource, "/")+1:]
}
