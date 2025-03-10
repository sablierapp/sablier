package kubernetes

import "context"

func (p *Provider) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	informer := p.watchDeployents(instance)
	go informer.Run(ctx.Done())
	informer = p.watchStatefulSets(instance)
	go informer.Run(ctx.Done())
}
