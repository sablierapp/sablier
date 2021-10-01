# Kubernetes

## Run the demo

# you need docker-compose, kubectl and helm (v3) installed

1. `git clone git@github.com:acouvreur/traefik-ondemand-plugin.git`
2. `cd traefik-ondemand-plugin/examples/kubernetes`
3. `docker-compose up`
4.  Wait 1 minute
5. `export KUBECONFIG=./kubeconfig.yaml`
5.  `helm repo add traefik https://helm.traefik.io/traefik`
6.  `helm repo update`
7.  Edit values.yaml and add your traefik pilot.token
8. `helm install traefik traefik/traefik -f values.yaml  --namespace kube-system   `
9.  `kubectl apply -f deploy-whoami.yml`
10.  `kubectl apply -f manifests.yml`
11.  Browse to http://localhost/ 
12. `kubectl get deployments -o wide`
```
NAME     READY   UP-TO-DATE   AVAILABLE   AGE   CONTAINERS   IMAGES              SELECTOR
whoami   1/1     1            1           16m   whoami       containous/whoami   app=whoami
```
13.  After 1 minute: `kubectl get deployments -o wide`
```
NAME     READY   UP-TO-DATE   AVAILABLE   AGE   CONTAINERS   IMAGES              SELECTOR
whoami   0/0     0            0           17m   whoami       containous/whoami   app=whoami`
```
14.  Browse to http://localhost/ 