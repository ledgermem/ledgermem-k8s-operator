# LedgerMem Kubernetes Operator

A controller-runtime operator that manages [LedgerMem](https://proofly.dev)
resources in your Kubernetes cluster.

## CRDs

- `LedgerMemCluster` — provisions a LedgerMem deployment (Deployment + Service +
  config) with a Postgres + pgvector / pinecone / qdrant backend.
- `Workspace` — declares a tenant workspace; reconciler calls the cluster's
  admin API to create it.
- `ApiKey` — declares an API key; reconciler creates it and writes the
  resulting plaintext token to a `Secret`.

## Quickstart

```sh
make deploy
kubectl apply -f config/samples/sample_cluster.yaml
kubectl get ledgermemclusters,workspaces,apikeys
```

## Develop

```sh
make build      # binary at ./bin/manager
make test       # unit tests + deepcopy round-trips
make docker-build IMG=ghcr.io/ledgermem/ledgermem-k8s-operator:dev
```

The reconcilers in this scaffold are intentionally minimal — extend them
to drive richer status, finalizers for cascading deletes, and webhook
admission for spec validation. See [`internal/controller/`](./internal/controller/).

## License

[Apache 2.0](./LICENSE)
