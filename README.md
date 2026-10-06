# Ginflix on Kubernetes

**Ginflix** is a small video streaming application (a mini Netflix): an
admin uploads videos, the backend converts them to HLS, and visitors watch
them in the browser.

This repository runs the whole thing on a local Kubernetes cluster
([kind](https://kind.sigs.k8s.io/)). Everything is inside the cluster,
there is no external service and no account to create, so anybody can
clone it and try it:

```bash
git clone <this repo> && cd ginflix-k8s
make up        # cluster + images + deploy + reverse proxy
make test
```

Then open http://ginflix.localhost:8088 (see [Use it](#use-it)).

The first run downloads about 3 GB of images, so it takes 10 to 30
minutes depending on the connection.

The application code in `app/` was written by Adrien Maes for the Télécom
Paris courses (GPL v3). The Kubernetes part is the subject of this
repository. See [Credits](#credits-and-license).

## Architecture

```
  browser
     |   http://ginflix.localhost:8088          public site + /api + /stream
     |   http://admin.ginflix.localhost:8088    admin site
     |   http://auth.ginflix.localhost:8088     login page (Keycloak)
     v
  Caddy reverse proxy (docker, on the "kind" network)
     |
     |  NodePort services
     v
 +---------------------- kind cluster, namespace "ginflix" ----------------------+
 |                                                                               |
 |  frontend x2     frontend-admin x1     keycloak x1                            |
 |                                           ^                                   |
 |                                           | public keys (JWKS)                |
 |  backend x1  -----------------------------+                                   |
 |     |    \                                                                    |
 |     |     +-------> seaweedfs x1 (S3) <------ streamer x2 (+ HPA)             |
 |     v                    |                                                    |
 |  mongodb x3 (rs0)       PVC                                                   |
 |     |                                                                         |
 |  3 PVC                                                                        |
 +-------------------------------------------------------------------------------+
```

| Component | Kubernetes objects | Role |
|---|---|---|
| frontend | Deployment (2), Service, ConfigMap | public web page, lists and plays the videos |
| frontend-admin | Deployment (1), Service, ConfigMap | admin page: upload and delete videos, after a login |
| backend | Deployment (1), Service, ConfigMap | REST API, converts uploads to HLS with ffmpeg, stores them in S3 |
| streamer | Deployment (2), Service, ConfigMap, HPA | reads the video files from S3 and sends them to the browser |
| mongodb | StatefulSet (3), headless Service, 3 PVC | video metadata, replica set `rs0` |
| mongo-init | Job | runs `rs.initiate()` once |
| seaweedfs | Deployment (1), Service, PVC | S3 storage for the video files |
| s3-init | Job | creates the bucket |
| keycloak | Deployment (1), Service, ConfigMap (realm) | login server (OpenID Connect) for the admin page |

## Repository

```
app/                      application source code (see Credits)
k8s/                      all the manifests (kubectl apply -k k8s)
  namespace.yaml
  mongodb.yaml            headless Service + StatefulSet
  mongo-init-job.yaml     Job that initiates the replica set
  seaweedfs.yaml          PVC + Deployment + Service + Job that creates the bucket
  keycloak.yaml           Deployment + Service
  keycloak-realm.json     realm, client and demo user imported by Keycloak
  backend.yaml            ConfigMap + Deployment + Service
  streamer.yaml           ConfigMap + Deployment + Service + HPA
  frontend.yaml           ConfigMap + Deployment + Service
  frontend-admin.yaml     ConfigMap + Deployment + Service
  network-policies.yaml
  kustomization.yaml      list of files + image tags
reverse-proxy/            Caddy (docker compose)
scripts/
  create-secrets.sh       creates the secrets from .env
  test.sh                 checks after the deployment
kind-config.yml           1 control plane + 2 workers, default CNI disabled
Makefile                  all the commands
```

## Requirements

- Docker (about 8 GB of free disk space and 6 GB of memory for Docker)
- [kind](https://kind.sigs.k8s.io/)
- kubectl
- make and bash (on Windows: WSL2 or Git Bash)

## Run it

`make up` does the five steps below in order. They can also be run one by
one.

| Step | Command | What it does |
|---|---|---|
| 1 | `make cluster` | creates the kind cluster (3 nodes) and installs Calico |
| 2 | `make metrics-server` | installs metrics-server, needed by the autoscaler |
| 3 | `make images` | builds the 4 images from `app/` and loads them in the nodes |
| 4 | `make deploy` | creates the secrets, applies `k8s/`, waits until everything is ready |
| 5 | `make proxy` | starts the reverse proxy on port 8088 |

There is no image registry: `kind load` copies the images directly into
the nodes.

The passwords come from the `.env` file. If it does not exist,
`make deploy` creates it from `.env.example` (demo values).

## Check it

```bash
make test
```

The script checks that:

1. all the deployments and the StatefulSet are ready
2. the MongoDB replica set has a primary
3. every service has pods behind it
4. the backend answers on `/api/videos` (so it can read MongoDB)
5. Keycloak gives a token to the demo user
6. a pod that is not allowed can **not** connect to MongoDB or SeaweedFS

## Use it

| URL | What |
|---|---|
| http://ginflix.localhost:8088 | public site |
| http://admin.ginflix.localhost:8088 | admin site, login `ginflix` / `ginflix` |
| http://auth.ginflix.localhost:8088 | Keycloak console, login in `.env` |

Browsers send every `*.localhost` name to your own machine, so there is
nothing to add in the hosts file.

To see a video on the public site:

1. open the admin site and log in
2. upload a short `.mp4` (a few seconds is better, the conversion is slow)
3. wait until the processing is finished
4. open the public site and play it

## Useful commands

```bash
make status          # pods, services, pvc, hpa, network policies
make logs            # backend logs
make mongo           # mongo shell on mongodb-0   (try: rs.status())
make clean           # delete the application
make cluster-delete  # delete the cluster
```

Some things to try:

```bash
# delete the primary and watch another member become primary
kubectl delete pod mongodb-0 -n ginflix
kubectl exec -n ginflix mongodb-1 -- mongosh --quiet --eval 'rs.status().members.map(m => m.name + " " + m.stateStr)'

# watch the autoscaler
kubectl get hpa -n ginflix -w
```

## Design choices

**StatefulSet + headless Service for MongoDB.** The members of a replica
set must know each other by name. A StatefulSet gives stable names
(`mongodb-0`, `mongodb-1`, `mongodb-2`) and one volume per pod, and the
headless Service gives each pod a DNS record (`mongodb-0.mongodb`).

**Jobs for the one-time setup.** Starting `mongod --replSet rs0` on three
pods is not enough, `rs.initiate()` has to be called once. A Job waits for
the three members and initiates the set only if it is not already done.
Another Job creates the S3 bucket. Both can be applied again without
breaking anything.

**Only one backend replica.** When a video is uploaded, the backend saves
it in its local `/tmp` and converts it in the background. The file only
exists in the pod that received the upload: with several replicas, the
"process again" button of the admin page can reach another pod, which does
not have the file. So the backend stays at 1 replica, and the autoscaler
is on the streamer, which keeps no state.

**Two kinds of URLs.** `BACKEND_URL`, `STREAM_URL` and `KEYCLOAK_URL` are
used by the JavaScript in the browser, so they are public addresses (the
reverse proxy). Inside the cluster the pods use service names: the backend
talks to `seaweedfs:8333`, to `mongodb-0.mongodb` and gets the Keycloak public
keys from `keycloak:8080`.

**NodePort services and a reverse proxy in a container.** The services
that the browser needs are `NodePort` with a fixed port. Caddy runs with
docker compose on the `kind` docker network, so it reaches the worker
nodes by their container name (`ginflix-worker:30080`). There is no IP
address to configure, and it works the same on Linux and on Docker
Desktop. SeaweedFS and MongoDB are `ClusterIP`: nothing outside needs them.

**Calico instead of the default CNI.** With the default network plugin of
kind, the network policies were accepted but not applied on Docker
Desktop for Windows: the test pod could still open a connection to
MongoDB. The cluster is created without the default CNI and Calico is
installed instead. `make test` checks that the policies really block.

**Network policies.** All incoming traffic is denied in the namespace,
then opened per application. MongoDB only accepts the backend (and its
init Job, and the other members). SeaweedFS only accepts the backend, the
streamer and its init Job.

**Security context.** Backend, streamer, MongoDB, SeaweedFS and Keycloak run
as a non-root user without Linux capabilities. Backend and streamer also
have a read-only root filesystem; the backend gets an `emptyDir` on `/tmp`
because it writes the uploads there.

**Secrets.** They are created with `kubectl create secret` from the `.env`
file, so there is no Secret YAML (base64 is not encryption) in git.

## Limitations

This is a demo on a local cluster, not a production setup:

- MongoDB has no authentication. It is only protected by the network policy.
- Keycloak runs in dev mode with its internal database, and the demo user
  and its password are in `k8s/keycloak-realm.json`.
- The application uses the admin key of SeaweedFS instead of a dedicated
  key limited to one bucket.
- Everything is plain HTTP, there is no TLS.
- The two frontend images are `nginx:alpine` running as root on port 80,
  so the namespace is in Pod Security `baseline`, not `restricted`.
- The backend cannot be scaled (see above). Fixing it needs a change in
  the application, or a shared volume.
- The backend has a 2 minute timeout on requests, so only short videos
  can be converted.
- SeaweedFS and Keycloak have a single replica.

## Credits and license

The application in `app/` (backend, streamer, frontend, frontend-admin)
was written by **Adrien Maes** (Télécom Paris) as teaching material and is
distributed under the GPL v3. Its original README is in `app/README.md`
and the license in `app/LICENSE`.

Changes made to the application so that it can run without the school
infrastructure:

- `backend/s3.go`, `streamer/config.go`: the variable `GARAGE_USE_SSL`
  (documented but not read) is now used, so S3 can be plain HTTP
- `backend/jwt.go`: new optional variable `KEYCLOAK_JWKS_URL`
- `frontend-admin`: the Keycloak address is a full URL (`KEYCLOAK_URL`)
  instead of `https://` + host
- `backend/Dockerfile`: multi-stage build (smaller image)
- `go.sum` files added with `go mod tidy` (needed by the Dockerfiles)

The rest of this repository is under the same license.
