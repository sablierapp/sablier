package awsecs

import (
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// listLabels hold a comma separated list. An ECS tag value cannot contain a
// comma, so on ECS these values are space separated and converted here.
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Tag.html
var listLabels = []string{sablier.LabelGroup, sablier.LabelRunningDays, sablier.LabelAntiAffinity}

// tagsToLabels converts the ECS tags of a service to the sablier label map.
func tagsToLabels(tags []types.Tag) map[string]string {
	labels := make(map[string]string, len(tags))
	for _, t := range tags {
		key := aws.ToString(t.Key)
		if key == "" {
			continue
		}
		value := aws.ToString(t.Value)
		if slices.Contains(listLabels, key) {
			value = strings.Join(strings.Fields(value), ",")
		}
		labels[key] = value
	}
	return labels
}
