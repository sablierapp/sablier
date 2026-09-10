package kubernetes

import (
	"sort"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func TestSablierConfig(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		want        map[string]string
	}{
		{
			name:   "labels only",
			labels: map[string]string{"sablier.enable": "true", "sablier.group": "myapp"},
			want:   map[string]string{"sablier.enable": "true", "sablier.group": "myapp"},
		},
		{
			name:        "annotations add sablier keys invalid as labels",
			labels:      map[string]string{"sablier.enable": "true"},
			annotations: map[string]string{"sablier.running-days": "Mon,Tue,Wed", "sablier.running-hours": "09:00-18:00"},
			want: map[string]string{
				"sablier.enable":        "true",
				"sablier.running-days":  "Mon,Tue,Wed",
				"sablier.running-hours": "09:00-18:00",
			},
		},
		{
			name:        "annotations override labels",
			labels:      map[string]string{"sablier.enable": "true", "sablier.group": "from-label"},
			annotations: map[string]string{"sablier.group": "from-annotation"},
			want:        map[string]string{"sablier.enable": "true", "sablier.group": "from-annotation"},
		},
		{
			name:        "non sablier annotations are ignored",
			labels:      map[string]string{"sablier.enable": "true"},
			annotations: map[string]string{"kubectl.kubernetes.io/last-applied-configuration": "{...}", "app": "demo"},
			want:        map[string]string{"sablier.enable": "true"},
		},
		{
			name:        "nil maps",
			labels:      nil,
			annotations: nil,
			want:        map[string]string{},
		},
		{
			name:   "public labels are read as sablier keys",
			labels: map[string]string{"sablierapp.dev/enable": "true", "sablierapp.dev/idle.replicas": "1"},
			want: map[string]string{
				"sablierapp.dev/enable":        "true",
				"sablierapp.dev/idle.replicas": "1",
				"sablier.enable":               "true",
				"sablier.idle.replicas":        "1",
			},
		},
		{
			name:        "public annotations are read as sablier keys",
			labels:      map[string]string{"sablierapp.dev/enable": "true"},
			annotations: map[string]string{"sablierapp.dev/running-hours": "09:00-18:00", "example.com/other": "demo"},
			want: map[string]string{
				"sablierapp.dev/enable": "true",
				"sablier.enable":        "true",
				"sablier.running-hours": "09:00-18:00",
			},
		},
		{
			name:   "public label overrides legacy label",
			labels: map[string]string{"sablier.group": "legacy", "sablierapp.dev/group": "public"},
			want:   map[string]string{"sablier.group": "public", "sablierapp.dev/group": "public"},
		},
		{
			name:        "public annotation overrides legacy annotation",
			annotations: map[string]string{"sablier.group": "legacy", "sablierapp.dev/group": "public"},
			want:        map[string]string{"sablier.group": "public"},
		},
		{
			name:        "legacy annotation overrides public label",
			labels:      map[string]string{"sablierapp.dev/group": "from-label"},
			annotations: map[string]string{"sablier.group": "from-annotation"},
			want:        map[string]string{"sablierapp.dev/group": "from-label", "sablier.group": "from-annotation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sablierConfig(tt.labels, tt.annotations)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestProvider_InstanceList_EnableKeys(t *testing.T) {
	t.Parallel()

	deployment := func(name string, labels map[string]string) *appsv1.Deployment {
		return &appsv1.Deployment{Namespace: "default", Name: name, Labels: labels}
	}
	statefulSet := func(name string, labels map[string]string) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{Namespace: "default", Name: name, Labels: labels}
	}

	p := newFakeCNPGProvider(t,
		newClusterObj("default", "pg-public", map[string]string{"sablierapp.dev/enable": "true", "sablierapp.dev/group": "db"}, nil, 1, 1),
		newClusterObj("default", "pg-legacy", map[string]string{"sablier.enable": "true", "sablier.group": "db"}, nil, 1, 1),
	)
	p.Client = k8sfake.NewSimpleClientset(
		deployment("public", map[string]string{"sablierapp.dev/enable": "true", "sablierapp.dev/group": "web"}),
		deployment("legacy", map[string]string{"sablier.enable": "true", "sablier.group": "web"}),
		deployment("both", map[string]string{"sablierapp.dev/enable": "true", "sablier.enable": "true"}),
		deployment("disabled", map[string]string{"sablierapp.dev/enable": "false"}),
		deployment("unlabeled", nil),
		statefulSet("public", map[string]string{"sablierapp.dev/enable": "true", "sablierapp.dev/group": "web"}),
		statefulSet("legacy", map[string]string{"sablier.enable": "true"}),
	)

	opts := ParseOptions{Delimiter: "_"}
	dPublic := DeploymentName(deployment("public", nil), opts).Original
	dLegacy := DeploymentName(deployment("legacy", nil), opts).Original
	dBoth := DeploymentName(deployment("both", nil), opts).Original
	ssPublic := StatefulSetName(statefulSet("public", nil), opts).Original
	ssLegacy := StatefulSetName(statefulSet("legacy", nil), opts).Original
	cPublic := ClusterName("default", "pg-public", opts).Original
	cLegacy := ClusterName("default", "pg-legacy", opts).Original

	got, err := p.InstanceList(t.Context(), provider.InstanceListOptions{All: true})
	assert.NilError(t, err)

	want := []sablier.InstanceConfiguration{
		{Name: dPublic, Groups: []string{"web"}, Enabled: "true"},
		{Name: dLegacy, Groups: []string{"web"}, Enabled: "true"},
		{Name: dBoth, Groups: []string{"default"}, Enabled: "true"},
		{Name: ssPublic, Groups: []string{"web"}, Enabled: "true"},
		{Name: ssLegacy, Groups: []string{"default"}, Enabled: "true"},
		{Name: cPublic, Groups: []string{"db"}, Enabled: "true"},
		{Name: cLegacy, Groups: []string{"db"}, Enabled: "true"},
	}
	sort.Slice(got, func(i, j int) bool { return strings.Compare(got[i].Name, got[j].Name) < 0 })
	sort.Slice(want, func(i, j int) bool { return strings.Compare(want[i].Name, want[j].Name) < 0 })
	assert.DeepEqual(t, got, want)

	groups, err := p.InstanceGroups(t.Context())
	assert.NilError(t, err)
	for _, instances := range groups {
		sort.Strings(instances)
	}

	wantGroups := map[string][]string{
		"web":     {dPublic, dLegacy, ssPublic},
		"default": {dBoth, ssLegacy},
		"db":      {cPublic, cLegacy},
	}
	for _, instances := range wantGroups {
		sort.Strings(instances)
	}
	assert.DeepEqual(t, groups, wantGroups)
}
