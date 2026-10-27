package awsecs

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func TestTagsToLabels(t *testing.T) {
	t.Parallel()

	tags := []types.Tag{
		{Key: aws.String(sablier.LabelEnable), Value: aws.String("true")},
		{Key: aws.String(sablier.LabelGroup), Value: aws.String("  team-a   team-b ")},
		{Key: aws.String(sablier.LabelRunningDays), Value: aws.String("Mon Tue")},
		{Key: aws.String(sablier.LabelAntiAffinity), Value: aws.String("streaming")},
		{Key: aws.String(sablier.LabelRunningHours), Value: aws.String("09:00-18:00")},
		{Key: aws.String("Environment"), Value: aws.String("prod")},
		{Key: aws.String("empty"), Value: nil},
		{Key: nil, Value: aws.String("ignored")},
	}

	got := tagsToLabels(tags)

	assert.DeepEqual(t, got, map[string]string{
		sablier.LabelEnable:       "true",
		sablier.LabelGroup:        "team-a,team-b",
		sablier.LabelRunningDays:  "Mon,Tue",
		sablier.LabelAntiAffinity: "streaming",
		sablier.LabelRunningHours: "09:00-18:00",
		"Environment":             "prod",
		"empty":                   "",
	})
}

func TestTagsToLabels_Empty(t *testing.T) {
	t.Parallel()

	assert.DeepEqual(t, tagsToLabels(nil), map[string]string{})
}
