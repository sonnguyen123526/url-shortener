# url-shortener

A small Go URL shortener backed by Redis, deployed on Kubernetes (kind).

## API

| Method | Path       | Description                                        |
|--------|------------|----------------------------------------------------|
| POST   | `/shorten` | Body `{"url": "https://..."}` → `201 {code, short_url}` |
| GET    | `/{code}`  | `302` redirect to the original URL, `404` if unknown |
| GET    | `/healthz` | Liveness: process is up                             |
| GET    | `/readyz`  | Readiness: Redis is reachable                       |

## Run on kind

```sh
kind create cluster --name learn   # once
make deploy                        # build image, load into kind, create secret, apply k8s/
make port-forward                  # in another terminal
```

Then:

```sh
curl -s -X POST localhost:8080/shorten -d '{"url":"https://go.dev"}'
curl -i localhost:8080/<code>
```

## Run with Helm

The same app packaged as a chart in `charts/url-shortener`, installed into the `url-shortener` namespace:

```sh
make helm-deploy                   # build image, load into kind, helm upgrade --install
make helm-port-forward
helm -n url-shortener get values url-shortener --all   # see what was installed
```

Common overrides (`--set key=value` or `-f my-values.yaml`): `replicaCount`, `baseURL`, `image.tag`, `redis.persistence.size`, `redis.auth.existingSecret`.

## Things to try

- **Persistence:** `kubectl delete pod redis-0`, wait for it to come back, and check that old codes still redirect.
- **Readiness:** `kubectl scale statefulset redis --replicas=0` and watch the app pods turn `0/1` (`kubectl get pods -w`). Scale back to 1.
- **Service discovery:** `kubectl run -it --rm dns --image=busybox:1.36 -- nslookup redis`.
- **Inspect the Secret:** `kubectl get secret redis-auth -o jsonpath='{.data.password}' | base64 -d` (base64 is encoding, not encryption).

## Layout

```
main.go                 config from env, graceful shutdown on SIGTERM
internal/handler        HTTP handlers + tests (in-memory store)
internal/store          Store interface, Redis and in-memory implementations
k8s/                    Deployment, Service, ConfigMap, Redis StatefulSet + headless Service
charts/url-shortener    Helm chart for the same resources
```

## Next steps

- Click counter per link (`INCR`) exposed as Prometheus metrics
- Ingress at `http://short.local`
- HorizontalPodAutoscaler + load test with `hey`
