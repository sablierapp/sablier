package kubernetes

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceStop(ctx context.Context, name string) (err error) {
	ctx, span := p.tracer.Start(ctx, "kubernetes.instance.stop",
		trace.WithAttributes(attribute.String("instance", name)))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	parsed, err := ParseName(name, ParseOptions{Delimiter: p.delimiter})
	if err != nil {
		return err
	}

	// CloudNativePG Clusters are stopped via the hibernation annotation rather than
	// scaling a replica count, so they bypass the scale-mode logic.
	if parsed.Kind == KindCNPGCluster {
		span.SetAttributes(attribute.String("operation", "cnpg_hibernate"))
		return p.clusterHibernate(ctx, parsed, true)
	}

	labels, err := p.getWorkloadLabels(ctx, parsed)
	if err != nil {
		return err
	}

	sc := sablier.ScaleConfigFromLabels(labels)
	if sc.Idle.Replicas >= 1 {
		span.SetAttributes(
			attribute.String("operation", "scale_mode"),
			attribute.Int("replicas", int(sc.Idle.Replicas)),
			attribute.String("cpu", sc.Idle.CPU),
			attribute.String("memory", sc.Idle.Memory),
		)
		p.l.DebugContext(ctx, "applying idle resources (scale mode)",
			slog.String("name", name),
			slog.Int("replicas", int(sc.Idle.Replicas)),
			slog.String("cpu", sc.Idle.CPU),
			slog.String("memory", sc.Idle.Memory),
		)
		if err := p.scale(ctx, parsed, sc.Idle.Replicas); err != nil {
			return err
		}
		if sc.Idle.CPU != "" || sc.Idle.Memory != "" {
			return p.scaleResources(ctx, parsed, sc.Idle.CPU, sc.Idle.Memory)
		}
		return nil
	}

	span.SetAttributes(attribute.String("operation", "scale_to_zero"))

	// For StatefulSets managed by the OT-CONTAINER-KIT redis-operator, pause the
	// operator's reconciliation loop before scaling to zero. This prevents the
	// operator from immediately restoring the replica count. The annotation is
	// cleared if the scale itself fails so the operator is never left paused with
	// pods still running.
	if parsed.Kind == "statefulset" {
		ss, fetchErr := p.Client.AppsV1().StatefulSets(parsed.Namespace).Get(ctx, parsed.Name, metav1.GetOptions{})
		if fetchErr != nil {
			p.l.WarnContext(ctx, "cannot fetch statefulset for redis-operator owner check",
				"namespace", parsed.Namespace, "name", parsed.Name, "error", fetchErr)
		} else if _, isRedis := redisOperatorOwner(ss); isRedis {
			p.setRedisOperatorSkipReconcile(ctx, ss, true)
			defer func() {
				if err != nil {
					// Scale failed: restore operator reconciliation using a detached
					// context so a cancelled request context doesn't prevent cleanup.
					p.setRedisOperatorSkipReconcile(context.WithoutCancel(ctx), ss, false)
				}
			}()
		}
	}

	return p.scale(ctx, parsed, 0)
}
