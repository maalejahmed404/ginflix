CLUSTER = ginflix
NS      = ginflix
TAG     = 1.0.0

CALICO_VERSION         = v3.30.3
METRICS_SERVER_VERSION = v0.7.2

.PHONY: help up cluster metrics-server images secrets deploy status test proxy proxy-down logs mongo clean cluster-delete

help:
	@echo "make up              do everything: cluster, images, deploy, proxy"
	@echo ""
	@echo "make cluster         create the kind cluster (with Calico)"
	@echo "make metrics-server  install metrics-server (needed by the HPA)"
	@echo "make images          build the 4 images and load them in the cluster"
	@echo "make secrets         create the secrets from .env"
	@echo "make deploy          deploy the application"
	@echo "make proxy           start the Caddy reverse proxy"
	@echo ""
	@echo "make status          show what is running"
	@echo "make test            run the checks"
	@echo "make logs            follow the backend logs"
	@echo "make mongo           open a mongo shell"
	@echo ""
	@echo "make proxy-down      stop the reverse proxy"
	@echo "make clean           delete the application (the volumes too)"
	@echo "make cluster-delete  delete the kind cluster"

up: cluster metrics-server images deploy proxy

cluster:
	kind create cluster --name $(CLUSTER) --config kind-config.yml
	# CNI plugin: pod network + NetworkPolicy. The nodes stay NotReady until it runs.
	kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/$(CALICO_VERSION)/manifests/calico.yaml
	kubectl rollout status daemonset/calico-node -n kube-system --timeout=600s
	kubectl wait --for=condition=Ready nodes --all --timeout=300s

metrics-server:
	kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/download/$(METRICS_SERVER_VERSION)/components.yaml
	# on kind the kubelet certificate is self-signed
	kubectl patch deployment metrics-server -n kube-system --type=json \
	  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'

# There is no registry: the images are built here and copied into the
# worker nodes with "kind load".
images:
	docker build -t ginflix/backend:$(TAG) app/backend
	docker build -t ginflix/streamer:$(TAG) app/streamer
	docker build -t ginflix/frontend:$(TAG) app/frontend
	docker build -t ginflix/frontend-admin:$(TAG) app/frontend-admin
	kind load docker-image --name $(CLUSTER) --nodes $(CLUSTER)-worker,$(CLUSTER)-worker2 \
	  ginflix/backend:$(TAG) ginflix/streamer:$(TAG) ginflix/frontend:$(TAG) ginflix/frontend-admin:$(TAG)

secrets:
	bash scripts/create-secrets.sh

deploy: secrets
	kubectl apply -k k8s
	kubectl rollout status statefulset/mongodb -n $(NS) --timeout=1200s
	kubectl wait --for=condition=complete job/mongo-init job/s3-init -n $(NS) --timeout=600s
	kubectl rollout status deployment/keycloak -n $(NS) --timeout=900s
	kubectl rollout status deployment/backend -n $(NS) --timeout=180s
	kubectl rollout status deployment/streamer -n $(NS) --timeout=180s
	kubectl rollout status deployment/frontend -n $(NS) --timeout=180s
	kubectl rollout status deployment/frontend-admin -n $(NS) --timeout=180s

status:
	kubectl get pods,svc,pvc,hpa,networkpolicy -n $(NS) -o wide

test:
	bash scripts/test.sh

proxy:
	docker compose -f reverse-proxy/docker-compose.yml up -d

proxy-down:
	docker compose -f reverse-proxy/docker-compose.yml down

logs:
	kubectl logs -n $(NS) -l app=backend -f --tail=100

mongo:
	kubectl exec -it -n $(NS) mongodb-0 -- mongosh

clean:
	kubectl delete namespace $(NS) --ignore-not-found

cluster-delete:
	kind delete cluster --name $(CLUSTER)
