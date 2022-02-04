# Command line tool for examininng K8s resources in Troublehsoot's support bundles

### How to use:

Set the path to support bundle archive or directory
```
export SBCTL_SUPPORT_BUNDLE_PATH=~/Downloads/support-bundle-2022-02-03T13_12_17
```

Run kubectl style commands

```
$ sbctl get ns
NAME                          STATUS   AGE
kube-system                   Active   204d
default                       Active   204d
kube-public                   Active   204d
kube-node-lease               Active   204d
docker-registry               Active   204d
schemahero-system             Active   199d
velero                        Active   135d
postgres-test                 Active   22d
redis-test                    Active   22d
nginx-test                    Active   22d
test                          Active   10d
```

```
$ sbctl get pods -n kube-system
NAME                                      READY   STATUS      RESTARTS       AGE
helm-install-traefik-crd-jk29f            0/1     Completed   0              204d
helm-install-traefik-nf68z                0/1     Completed   1 (204d ago)   204d
svclb-traefik-clk94                       2/2     Running     2 (165d ago)   204d
metrics-server-86cbb8457f-g9kdc           1/1     Running     1 (165d ago)   204d
local-path-provisioner-5ff76fc89d-cgpdv   1/1     Running     2 (86d ago)    204d
coredns-7448499f4d-x8fw9                  1/1     Running     1 (165d ago)   204d
traefik-97b44b794-mhvsh                   1/1     Running     1 (165d ago)   204d
```