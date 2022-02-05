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

```
$ ./bin/sbctl describe pod kotsadm-75d9ff6f44-ssrx6 
Name:         kotsadm-75d9ff6f44-ssrx6
Namespace:    default
Priority:     0
Node:         my-node/***HIDDEN***
Start Time:   Tue, 01 Feb 2022 18:31:36 -0800
Labels:       app=kotsadm
              app.kubernetes.io/name=kotsadm
              kots.io/backup=velero
              kots.io/kotsadm=true
              pod-template-hash=75d9ff6f44
              skaffold.dev/run-id=ca77ed45-8a57-44af-ac0f-ec1931c57841
Annotations:  backup.velero.io/backup-volumes: backup
              pre.hook.backup.velero.io/command:
                ["/bin/bash", "-c", "PGPASSWORD=password pg_dump -U kotsadm -h kotsadm-postgres > /backup/kotsadm-postgres.sql"]
              pre.hook.backup.velero.io/timeout: 3m
Status:       Running
IP:           ***HIDDEN***
IPs:
  IP:           ***HIDDEN***
Controlled By:  ReplicaSet/kotsadm-75d9ff6f44
Containers:
  kotsadm:
    Container ID:   containerd://84288b23eaf84112248eea8ec2f94a0f8f231036a46715c936c741154173271d
    Image:          localhost:32000/kotsadm:v1.60.0-26-g4e016d2ff-dirty@sha256:6c2f016f1e99a1f8b2129eb6b93ba59526118bdcada8c8e73f051db4123ff683
    Image ID:       localhost:32000/kotsadm@sha256:6c2f016f1e99a1f8b2129eb6b93ba59526118bdcada8c8e73f051db4123ff683
    Ports:          40000/TCP, 3000/TCP, 9229/TCP
    Host Ports:     0/TCP, 0/TCP, 0/TCP
    State:          Running
      Started:      Tue, 01 Feb 2022 18:31:39 -0800
    Ready:          True
    Restart Count:  0
    Limits:
      cpu:     1
      memory:  2Gi
    Requests:
      cpu:     100m
      memory:  100Mi
    Environment:
      POSTGRES_URI:               <set to the key 'uri' in secret 'kotsadm-postgres'>  Optional: false
      S3_BUCKET_NAME:             shipbucket
      S3_ENDPOINT:                http://kotsadm-s3:4569/
      S3_ACCESS_KEY_ID:           ***HIDDEN***
      S3_SECRET_ACCESS_KEY:       ***HIDDEN***
      S3_BUCKET_ENDPOINT:         true
      DEX_PGPASSWORD:             <set to the key 'PGPASSWORD' in secret 'kotsadm-dex-postgres'>  Optional: false
      KOTSADM_LOG_LEVEL:          debug
      DISABLE_SPA_SERVING:        1
      KOTSADM_TARGET_NAMESPACE:   test
      AUTO_CREATE_CLUSTER:        1
      AUTO_CREATE_CLUSTER_NAME:   microk8s
      AUTO_CREATE_CLUSTER_TOKEN:  ***HIDDEN***
      POD_NAMESPACE:              default (v1:metadata.namespace)
      SHARED_PASSWORD_BCRYPT:     ***HIDDEN***
      SESSION_KEY:                this-is-not-too-secret
      API_ENCRYPTION_KEY:         IvWItkB8+ezMisPjSMBknT1PdKjBx7Xc/txZqOP8Y2Oe7+Jy
      REPLICATED_API_ENDPOINT:    http://replicated-app:3000
      API_ENDPOINT:               http://kotsadm:3000
      API_ADVERTISE_ENDPOINT:     http://***HIDDEN***:30000
      KOTSADM_ENV:                dev
      ENABLE_WEB_PROXY:           1
      KURL_PROXY_TLS_CERT_PATH:   /etc/kurl-proxy/ca/tls.crt
      KOTS_INSTALL_ID:            dev-1pu4oeY162e2pbLpK4JubK6hxrX
      AIRGAP_UPLOAD_PARALLELISM:  3
      POD_OWNER_KIND:             deployment
      DEBUG:                      false
    Mounts:
      /backup from backup (rw)
      /etc/kubernetes/pki/kubelet from kubelet-client-cert (rw)
      /var/run/secrets/kubernetes.io/serviceaccount from kube-api-access-zfw8v (ro)
Conditions:
  Type              Status
  Initialized       True 
  Ready             True 
  ContainersReady   True 
  PodScheduled      True 
Volumes:
  backup:
    Type:       EmptyDir (a temporary directory that shares a pod's lifetime)
    Medium:     Memory
    SizeLimit:  1Gi
  kubelet-client-cert:
    Type:        Secret (a volume populated by a Secret)
    SecretName:  kubelet-client-cert
    Optional:    true
  init-dex-db:
    Type:      ConfigMap (a volume populated by a ConfigMap)
    Name:      init-dex-db
    Optional:  false
  kube-api-access-zfw8v:
    Type:                    Projected (a volume that contains injected data from multiple sources)
    TokenExpirationSeconds:  3607
    ConfigMapName:           kube-root-ca.crt
    ConfigMapOptional:       <nil>
    DownwardAPI:             true
QoS Class:                   Burstable
Node-Selectors:              <none>
Tolerations:                 node.kubernetes.io/not-ready:NoExecute op=Exists for 300s
                             node.kubernetes.io/unreachable:NoExecute op=Exists for 300s
```