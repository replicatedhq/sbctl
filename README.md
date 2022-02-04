# Command line tool for examininng K8s resources in Troublehsoot's support bundles

### How to use:

Set the path to support bundle archive to directory
```
export SBCTL_SUPPORT_BUNDLE_PATH=~/Downloads/support-bundle-2022-02-03T13_12_17
```

Run kubectl style commands

```
$ sbctl get ns
NAME                          AGE
kube-system                   203d
default                       203d
kube-public                   203d
kube-node-lease               203d
docker-registry               203d
schemahero-system             198d
velero                        135d
postgres-test                 22d
redis-test                    22d
nginx-test                    22d
test                          10d
```

```
$ sbctl get pods -n kube-system
NAME                                      AGE
helm-install-traefik-crd-jk29f            203d
helm-install-traefik-nf68z                203d
svclb-traefik-clk94                       203d
metrics-server-86cbb8457f-g9kdc           203d
local-path-provisioner-5ff76fc89d-cgpdv   203d
coredns-7448499f4d-x8fw9                  203d
traefik-97b44b794-mhvsh                   203d
```