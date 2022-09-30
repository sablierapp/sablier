# Kubernetes traefik-ondemand-service Howto

# Traefik parameters

Its important to set allowEmptyServices to true, otherwhise the scale up will
not work because traefik cannot find the service if it was scaled down to zero.

      - "--pilot.token=xxxx"
      - "--experimental.plugins.traefik-ondemand-plugin.modulename=github.com/acouvreur/traefik-ondemand-plugin/plugins/traefik"
      - "--experimental.plugins.traefik-ondemand-plugin.version=v0.1.1"
      - "--providers.kubernetesingress.allowEmptyServices=true"

 If you are using the traefik helm chart its also important to set:

     experimental:
      plugins:
        enabled: true

# Deployment

In this example we will deploy the traefik-ondemand-service into the namespace kube-system

    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: traefik-ondemand-service
      namespace: kube-system
      labels:
        app: traefik-ondemand-service
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: traefik-ondemand-service
      template:
        metadata:
          labels:
            app: traefik-ondemand-service
        spec:
          serviceAccountName: traefik-ondemand-service
          serviceAccount: traefik-ondemand-service
          containers:
          - name: traefik-ondemand-service
            image: gchr.io/acouvreur/traefik-ondemand-service
            args: ["--swarmMode=false", "--kubernetesMode=true"]
            ports:
            - containerPort: 10000
    ---
    apiVersion: v1
    kind: Service
    metadata:
      name: traefik-ondemand-service
      namespace: kube-system
    spec:
      selector:
        app: traefik-ondemand-service
      ports:
        - protocol: TCP
          port: 10000
          targetPort: 10000

We have to create RBAC to allow the traefik-ondemand-service to access the kubernetes API and get/update/patch the deployment resource

    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: traefik-ondemand-service
      namespace: kube-system
    ---
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: traefik-ondemand-service
      namespace: kube-system
    rules:
      - apiGroups:
          - apps
        resources:
          - statefulsets
          - statefulsets/scale
          - deployments
          - deployments/scale
        verbs:
          - patch
          - get
          - update
    ---
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: traefik-ondemand-service
      namespace: kube-system
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: traefik-ondemand-service
    subjects:
      - kind: ServiceAccount
        name: traefik-ondemand-service
        namespace: kube-system

## Creating a Middleware

In this example we want to scale down the `code-server` deployment in the `codeserverns` namespace
First we need to create a traefik middleware for that:

    apiVersion: traefik.containo.us/v1alpha1
    kind: Middleware
    metadata:
      name: ondemand-codeserver
      namespace: kube-system
    spec:
      plugin:
        traefik-ondemand-plugin:
          name: deployment_codeserverns_code-server_1
          serviceUrl: 'http://traefik-ondemand-service:10000'
          timeout: 10m

The format of the `name:` section is `<KIND>_<NAMESPACE>_<NAME>_<REPLICACOUNT>` where `_` is the delimiter.

`KIND` can be either `deployment` or `statefulset`

## Using the Middleware

When using an Ingress (e.g. for code-server) you have to add the middleware in metadata.annotation:

    traefik.ingress.kubernetes.io/router.middlewares: kube-system-ondemand-codeserver@kubernetescrd
