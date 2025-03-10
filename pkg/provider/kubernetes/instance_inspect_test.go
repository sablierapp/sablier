package kubernetes_test

import (
	"context"
	"fmt"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"gotest.tools/v3/assert"
	"testing"
)

func TestKubernetesProvider_InstanceInspect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ctx := context.Background()
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "invalid format name",
			args: args{
				name: "invalid-name-format",
			},
			want: fmt.Errorf("invalid name [invalid-name-format] should be: kind_namespace_name_replicas"),
		},
		{
			name: "unsupported resource name",
			args: args{
				name: "service_default_my-service_1",
			},
			want: fmt.Errorf("unsupported kind \"service\" must be one of \"deployment\", \"statefulset\""),
		},
	}
	c := setupKinD(t, ctx)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := kubernetes.New(ctx, c.client, slogt.New(t), config.NewProviderConfig().Kubernetes)

			_, err = p.InstanceInspect(ctx, tt.args.name)
			assert.Error(t, err, tt.want.Error())
		})
	}
}
