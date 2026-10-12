KIND_CLUSTER ?= learn
IMAGE        ?= url-shortener:dev
HELM_NS      ?= url-shortener

.PHONY: test run image kind-load secret deploy rollout port-forward logs undeploy helm-lint helm-deploy helm-port-forward helm-undeploy

test:
	go vet ./...
	go test -race ./...

# Run locally against a Redis on localhost:6379 (e.g. `docker run -p 6379:6379 redis:7.4-alpine`).
run:
	go run .

image:
	docker build -t $(IMAGE) .

# kind nodes can't see your local Docker images; copy the image into the cluster.
kind-load: image
	kind load docker-image $(IMAGE) --name $(KIND_CLUSTER)

# Create the Redis password once with a random value. It's not recreated on later runs,
# because Redis would keep the old password and the app would get the new one.
secret:
	@kubectl get secret redis-auth >/dev/null 2>&1 || \
		kubectl create secret generic redis-auth --from-literal=password=$$(openssl rand -hex 16)

deploy: kind-load secret
	kubectl apply -k k8s/
	$(MAKE) rollout

# Pods keep the old image when the tag doesn't change, so force a restart after rebuilding.
rollout:
	kubectl rollout restart deployment/url-shortener
	kubectl rollout status deployment/url-shortener --timeout=120s

port-forward:
	kubectl port-forward service/url-shortener 8080:80

logs:
	kubectl logs -l app=url-shortener -f --prefix

# Leaves the Redis PVC (data-redis-0) in place. Delete it with:
#   kubectl delete pvc data-redis-0
undeploy:
	kubectl delete -k k8s/ --ignore-not-found
	kubectl delete secret redis-auth --ignore-not-found

# ---- Helm (installs into its own namespace so it doesn't clash with the k8s/ deploy) ----

helm-lint:
	helm lint charts/url-shortener --strict
	helm template url-shortener charts/url-shortener >/dev/null

helm-deploy: kind-load
	helm upgrade --install url-shortener charts/url-shortener \
		--namespace $(HELM_NS) --create-namespace --wait --timeout 2m
	# Same image tag as before, so restart to pick up the rebuilt image.
	kubectl -n $(HELM_NS) rollout restart deployment/url-shortener
	kubectl -n $(HELM_NS) rollout status deployment/url-shortener --timeout=120s

helm-port-forward:
	kubectl -n $(HELM_NS) port-forward service/url-shortener 8080:80

# Keeps the Redis PVC; see the uninstall notes printed by `helm-deploy`.
helm-undeploy:
	helm uninstall url-shortener --namespace $(HELM_NS) --ignore-not-found
