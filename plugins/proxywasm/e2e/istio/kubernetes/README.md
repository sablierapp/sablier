# Install kubectl
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install helm3
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Start k3s
docker compose up -d
sudo chown vscode ./kubeconfig.yaml
chmod 600 ./kubeconfig.yaml
export KUBECONFIG=./kubeconfig.yaml

kubectl create configmap -n istio-system sablier-wasm-plugin --from-file ../../sablierproxywasm.wasm

# Install Istio Helm charts
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update
helm install istio-base istio/base -n istio-system --wait
helm install istiod istio/istiod -n istio-system --wait
kubectl label namespace istio-system istio-injection=enabled
helm install istio-ingressgateway istio/gateway --values ./istio-gateway-values.yaml -n istio-system --wait

# Install Sablier
kubectl apply -f ./manifests/sablier.yml

# Build proxywasm
make docker