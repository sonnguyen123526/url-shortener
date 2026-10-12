# url-shortener

A small Go URL shortener backed by Redis, deployed on Kubernetes (kind) with a Helm chart.
Links survive restarts because Redis runs as a StatefulSet with a PersistentVolumeClaim.

## API

| Method | Path       | Description                                             |
|--------|------------|---------------------------------------------------------|
| POST   | `/shorten` | Body `{"url": "https://..."}` → `201 {code, short_url}` |
| GET    | `/{code}`  | `302` redirect to the original URL, `404` if unknown    |
| GET    | `/healthz` | Liveness: process is up                                 |
| GET    | `/readyz`  | Readiness: Redis is reachable                           |

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (running)
- [kind](https://kind.sigs.k8s.io/), [kubectl](https://kubernetes.io/docs/tasks/tools/), [Helm](https://helm.sh/), `make`, `openssl`
- Go 1.27+ (only for running tests or the app outside Docker)

On macOS: `brew install kind kubectl helm go`

## Quickstart

**1. Start Docker and wait until it's ready**

```sh
open -a Docker    # macOS; on Linux make sure the Docker daemon is running
until docker info >/dev/null 2>&1; do sleep 2; done; echo "Docker is ready"
```

**2. Get the code**

```sh
git clone https://github.com/<your-github-user>/url-shortener.git
cd url-shortener
```

**3. Create a kind cluster and point kubectl at it**

```sh
kind get clusters                  # skip the next line if "learn" is listed
kind create cluster --name learn
kubectl config use-context kind-learn
kubectl get nodes                  # wait for STATUS = Ready
```

The Makefile deploys to whatever kubectl context is active, so make sure it's `kind-learn`.
To use a cluster with another name, pass it to every make command: `make helm-deploy KIND_CLUSTER=<name>`.

**4. Build and deploy**

```sh
make helm-deploy    # builds the image, loads it into kind, runs helm upgrade --install
kubectl -n url-shortener get pods
```

You should see three pods, all `1/1 Running`:

```
NAME                             READY   STATUS    RESTARTS   AGE
url-shortener-5749b5868b-4xpzt   1/1     Running   0          95s
url-shortener-5749b5868b-gtr6f   1/1     Running   0          96s
url-shortener-redis-0            1/1     Running   0          103s
```

**5. Open the API** (in a second terminal; leave it running)

```sh
make helm-port-forward
```

**6. Try it** (back in the first terminal)

```sh
curl -s -X POST localhost:8080/shorten -d '{"url":"https://go.dev"}'
# {"code":"KuDwRhP","short_url":"http://localhost:8080/KuDwRhP"}

curl -i localhost:8080/KuDwRhP     # use your own code
# HTTP/1.1 302 Found
# Location: https://go.dev
```

You can also open the `short_url` in a browser.

## Things to try

All commands assume the Helm install in the `url-shortener` namespace.

- **Persistence:** delete the Redis pod, wait for it to come back, and check that old codes still redirect.
  ```sh
  kubectl -n url-shortener delete pod url-shortener-redis-0
  kubectl -n url-shortener get pods -w      # wait for 1/1 Running, then Ctrl+C
  curl -i localhost:8080/<code>
  ```
- **Readiness:** stop Redis and watch the app pods turn `0/1` (they stop receiving traffic but aren't restarted).
  ```sh
  kubectl -n url-shortener scale statefulset url-shortener-redis --replicas=0
  kubectl -n url-shortener get pods -w
  kubectl -n url-shortener scale statefulset url-shortener-redis --replicas=1
  ```
- **Service discovery:** the app finds Redis through cluster DNS.
  ```sh
  kubectl -n url-shortener run -it --rm dns --image=busybox:1.36 --restart=Never -- nslookup url-shortener-redis
  ```
- **Inspect the Secret** (base64 is encoding, not encryption):
  ```sh
  kubectl -n url-shortener get secret url-shortener-redis -o jsonpath='{.data.password}' | base64 -d
  ```
- **Logs:**
  ```sh
  kubectl -n url-shortener logs -l app.kubernetes.io/component=app -f --prefix
  ```

## Configuration

Override chart values with `--set key=value` or `-f my-values.yaml` (see `charts/url-shortener/values.yaml`):

| Value                       | Default                 | Description                                         |
|-----------------------------|-------------------------|-----------------------------------------------------|
| `replicaCount`              | `2`                     | App pods                                            |
| `baseURL`                   | `http://localhost:8080` | Base of the returned short links                    |
| `image.repository` / `.tag` | `url-shortener` / `dev` | App image                                           |
| `redis.auth.password`       | empty (random)          | Generated password is kept across `helm upgrade`    |
| `redis.auth.existingSecret` | empty                   | Use your own Secret with a `password` key           |
| `redis.persistence.size`    | `1Gi`                   | Redis PVC size                                      |

See what's installed: `helm -n url-shortener get values url-shortener --all`

## Development

```sh
make test          # go vet + tests with the race detector (uses an in-memory store, no Redis needed)

# Run the app locally against Redis in Docker
docker run -d --name redis -p 6379:6379 redis:7.4-alpine
make run           # http://localhost:8080
```

After changing code, `make helm-deploy` rebuilds the image and restarts the pods.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `failed to connect to the docker API` | Docker Desktop isn't running; see step 1. |
| `kind create cluster` says the cluster already exists | Fine. Skip it and run `kubectl config use-context kind-learn`. |
| `kubectl get nodes` hangs or connection refused | The kind node didn't restart with Docker: `docker start learn-control-plane`. |
| `curl` fails after a pod restart | The port-forward dropped. Ctrl+C it and run `make helm-port-forward` again. |
| App pods stuck `0/1 Running` | Redis isn't ready yet; check `kubectl -n url-shortener get pods`. |
| `make helm-deploy` times out | Check `kubectl -n url-shortener get events --sort-by=.lastTimestamp`, then `make helm-undeploy && make helm-deploy`. |

## Clean up

```sh
make helm-undeploy                                              # removes the release
kubectl -n url-shortener delete pvc data-url-shortener-redis-0  # deletes stored links
kubectl delete namespace url-shortener
kind delete cluster --name learn                                # optional: remove the cluster
```

## Alternative: plain manifests (no Helm)

`k8s/` has the same resources as plain YAML, applied with Kustomize into the `default` namespace.
Resource names differ from the Helm install: `redis`, `redis-0`, and Secret `redis-auth`.

```sh
make deploy          # build, load into kind, create a random Redis password Secret, kubectl apply -k k8s/
make port-forward    # same port 8080, so don't run it alongside make helm-port-forward
make undeploy        # keeps the PVC; delete it with: kubectl delete pvc data-redis-0
```

## Layout

```
main.go                 config from env, graceful shutdown on SIGTERM
internal/handler        HTTP handlers + tests (in-memory store)
internal/store          Store interface, Redis and in-memory implementations
charts/url-shortener    Helm chart
k8s/                    The same resources as plain manifests (Kustomize)
Makefile                build, deploy, port-forward, clean-up shortcuts
```

## Next steps

- Click counter per link (`INCR`) exposed as Prometheus metrics
- Ingress at `http://short.local`
- HorizontalPodAutoscaler + load test with `hey`
