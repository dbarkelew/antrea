# Antrea Multi-cluster User Guide

## Table of Contents

<!-- toc -->
- [Quick Start](#quick-start)
- [Installation](#installation)
  - [Preparation](#preparation)
  - [Deploy Antrea Multi-cluster Controller](#deploy-antrea-multi-cluster-controller)
  - [Create ClusterSet](#create-clusterset)
- [Multi-cluster Gateway Configuration](#multi-cluster-gateway-configuration)
- [Multi-cluster Service](#multi-cluster-service)
- [Multi-cluster ClusterNetworkPolicy Replication](#multi-cluster-clusternetworkpolicy-replication)
- [Build Antrea Multi-cluster Image](#build-antrea-multi-cluster-image)
- [Known Issue](#known-issue)
<!-- /toc -->

Antrea Multi-cluster implements [Multi-cluster Service API](https://github.com/kubernetes/enhancements/tree/master/keps/sig-multicluster/1645-multi-cluster-services-api),
which allows users to create multi-cluster Services that can be accessed cross
clusters in a ClusterSet. Antrea Multi-cluster also supports Antrea
ClusterNetworkPolicy replication. A Multi-cluster ClusterSet admin can define
ClusterNetworkPolicies to be replicated across the entire ClusterSet and
enforced in all member clusters. Antrea Multi-cluster was first introduced in
Antrea v1.5.0, and the ClusterNetworkPolicy replication feature is supported
since Antrea v1.6.0. In Antrea v1.7.0, the Multi-cluster Gateway feature was
added that supports routing multi-cluster Service traffic through tunnels among
clusters.

## Quick Start

Please refer to the [Quick Start Guide](./quick-start.md) to learn how to build a ClusterSet
with two clusters quickly.

## Installation

### Preparation

We assume an Antrea version >= `v1.8.0` is used in this guide, and the Antrea
version is set to an environment variable `TAG`. For example, the following
command sets the Antrea version to `v1.8.0`.

```bash
export TAG=v1.8.0
```

To use the latest version of Antrea Multi-cluster from the Antrea main branch,
you can change the YAML manifest path to: `https://github.com/antrea-io/antrea/tree/main/multicluster/build/yamls/`
when applying or downloading an Antrea YAML manifest.

Since Antrea 1.7.0, multi-cluster Services require an Antrea Multi-cluster
Gateway to be set up in each member cluster to route Service traffic across
clusters, and Service CIDRs (ClusterIP ranges) must not overlap among clusters.
To support Multi-cluster Gateways, `antrea-agent` must be deployed with the
`Multicluster` feature enabled in a member cluster. You can set the following
configuration parameters in `antrea-agent.conf` of the Antrea deployment
manifest to enable the `Multicluster` feature:

```yaml
antrea-agent.conf: |
...
  featureGates:
...
    Multicluster: true
...
  multicluster:
    enable: true
    namespace: ""
```

At the moment, Multi-cluster Gateway only works with the Antrea `encap` traffic
mode, and all member clusters in a ClusterSet must use the same tunnel type.

### Deploy Antrea Multi-cluster Controller

A Multi-cluster ClusterSet is comprised of a single leader cluster and at least
two member clusters. Antrea Multi-cluster Controller needs to be deployed in the
leader and all member clusters. A cluster can serve as the leader, and meanwhile
also be a member cluster of the ClusterSet. To deploy Multi-cluster Controller
in a dedicated leader cluster, please refer to [Deploy in a Dedicated Leader
cluster](#deploy-in-a-dedicated-leader-cluster). To deploy Multi-cluster
Controller in a member cluster, please refer to [Deploy in a Member Cluster](#deploy-in-a-member-cluster).
To deploy Multi-cluster Controller in a dual-role cluster, please refer to
[Deploy Leader and Member in One Cluster](#deploy-leader-and-member-in-one-cluster).

#### Deploy in a Dedicated Leader Cluster

1. Run the following command to import Multi-cluster CRDs in the leader cluster:

     ```bash
     kubectl apply -f https://github.com/antrea-io/antrea/releases/download/$TAG/antrea-multicluster-leader-global.yml
     ```

2. Install Multi-cluster Controller in the leader cluster. Since Multi-cluster
   Controller runs as a namespaced Deployment, you should create the Namespace
   first, and then apply the deployment manifest with the Namespace.

  ```bash
  kubectl create ns antrea-multicluster
  kubectl apply -f https://github.com/antrea-io/antrea/releases/download/$TAG/antrea-multicluster-leader-namespaced.yml
  ```

The Multi-cluster Controller in the leader cluster will be deployed in Namespace `antrea-multicluster`
by default. If you'd like to use another Namespace, you can change `antrea-multicluster` to the desired
Namespace in `antrea-multicluster-leader-namespaced.yml`, for example:

```bash
$kubectl create ns '<desired-namespace>'
$curl -L https://github.com/antrea-io/antrea/releases/download/$TAG/antrea-multicluster-leader-namespaced.yml > antrea-multicluster-leader-namespaced.yml
$sed 's/antrea-multicluster/<desired-namespace>/g' antrea-multicluster-leader-namespaced.yml | kubectl apply -f -
```

#### Deploy in a Member Cluster

You can run the following command to install Multi-cluster Controller in a
member cluster. The command will run the controller in the "member" mode in the
`kube-system` Namespace. If you want to use a different Namespace other than
`kube-system`, you can edit `antrea-multicluster-member.yml` and change
`kube-system` to the desired Namespace.

```bash
kubectl apply -f https://github.com/antrea-io/antrea/releases/download/$TAG/antrea-multicluster-member.yml
```

#### Deploy Leader and Member in One Cluster

We need to run two instances of Multi-cluster Controller in the dual-role
cluster, one in leader mode and another in member mode.

1. Follow the steps in section [Deploy in a Dedicated Leader Cluster](#deploy-in-a-dedicated-leader-cluster)
   to deploy the leader controller and import the Multi-cluster CRDs.
2. Follow the steps in section [Deploy in a Member Cluster](#deploy-in-a-member-cluster)
   to deploy the member controller.

### Create ClusterSet

An Antrea Multi-cluster ClusterSet should include at least one leader cluster
and two member clusters. As an example, in the following sections we will create
a ClusterSet `test-clusterset` which has two member clusters with cluster ID
`test-cluster-east` and `test-cluster-west` respectively, and one leader cluster
with ID `test-cluster-north`.

#### Set up Access to Leader Cluster

We first need to set up access to the leader cluster's API server for all member
clusters. We recommend creating one ServiceAccount for each member for
fine-grained access control.

1. Apply the following YAML manifest in the leader cluster to set up access for
   `test-cluster-east`:

   ```yml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: member-east
     namespace: antrea-multicluster
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: member-east-token
     namespace: antrea-multicluster
     annotations:
       kubernetes.io/service-account.name: member-east
   type: kubernetes.io/service-account-token
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: member-east
     namespace: antrea-multicluster
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: Role
     name: antrea-mc-member-cluster-role
   subjects:
     - kind: ServiceAccount
       name: member-east
       namespace: antrea-multicluster
   ```

2. Generate the token Secret manifest from the leader cluster, and create a
   Secret with the manifest in member cluster `test-cluster-east`, e.g.:

   ```bash
   # Generate the file 'member-east-token.yml' from your leader cluster
   kubectl get secret member-east-token -n antrea-multicluster -o yaml | grep -w -e '^apiVersion' -e '^data' -e '^metadata' -e '^ *name:'  -e   '^kind' -e '  ca.crt' -e '  token:' -e '^type' -e '  namespace' | sed -e 's/kubernetes.io\/service-account-token/Opaque/g' -e 's/antrea-multicluster/kube-system/g' >  member-east-token.yml
   # Apply 'member-east-token.yml' to the member cluster.
   kubectl apply -f member-east-token.yml --kubeconfig=/path/to/kubeconfig-of-member-test-cluster-east
   ```

3. Replace all `east` to `west` and repeat step 1/2 for the other member cluster
   `test-cluster-west`.

#### Initialize ClusterSet

In all clusters, a `ClusterSet` CR must be created to define the ClusterSet, and
two `ClusterClaim` CRs must be created to claim the ClusterSet and claim the
cluster is a member of the ClusterSet.

- Create `ClusterClaim` and `ClusterSet` in the leader cluster
`test-cluster-north` with the following YAML manifest (you can also refer to
[leader-clusterset-template.yml](../../multicluster/config/samples/clusterset_init/leader-clusterset-template.yml)):

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha2
kind: ClusterClaim
metadata:
  name: id.k8s.io
  namespace: antrea-multicluster
value: test-cluster-north
---
apiVersion: multicluster.crd.antrea.io/v1alpha2
kind: ClusterClaim
metadata:
  name: clusterset.k8s.io
  namespace: antrea-multicluster
value: test-clusterset
---
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterSet
metadata:
  name: test-clusterset
  namespace: antrea-multicluster
spec:
  leaders:
    - clusterID: test-cluster-north
```

- Create `ClusterClaim` and `ClusterSet` in member cluster `test-cluster-east`
with the following YAML manifest (you can also refer to
[member-clusterset-template.yml](../../multicluster/config/samples/clusterset_init/member-clusterset-template.yml)):

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha2
kind: ClusterClaim
metadata:
  name: id.k8s.io
  namespace: kube-system
value: test-cluster-east
---
apiVersion: multicluster.crd.antrea.io/v1alpha2
kind: ClusterClaim
metadata:
  name: clusterset.k8s.io
  namespace: kube-system
value: test-clusterset
---
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterSet
metadata:
  name: test-clusterset
  namespace: kube-system
spec:
  leaders:
    - clusterID: test-cluster-north
      secret: "member-east-token"
      server: "https://172.18.0.1:6443"
  namespace: antrea-multicluster
```

Note: update `server: "https://172.18.0.1:6443"` in the `ClusterSet` spec to the
correct leader cluster API server address.

- Create `ClusterClaim` and `ClusterSet` in member cluster `test-cluster-west`:

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha2
kind: ClusterClaim
metadata:
  name: id.k8s.io
  namespace: kube-system
value: test-cluster-west
---
apiVersion: multicluster.crd.antrea.io/v1alpha2
kind: ClusterClaim
metadata:
  name: clusterset.k8s.io
  namespace: kube-system
value: test-clusterset
---
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterSet
metadata:
  name: test-clusterset
  namespace: kube-system
spec:
  leaders:
    - clusterID: test-cluster-north
      secret: "member-west-token"
      server: "https://172.18.0.1:6443"
  namespace: antrea-multicluster
```

#### Initialize ClusterSet for a Dual-role Cluster

If you want to make the leader cluster `test-cluster-north` also a member
cluster of the ClusterSet, make sure you follow the steps in [Deploy Leader and
Member in One Cluster](#deploy-leader-and-member-in-one-cluster) and repeat the
steps in [Set up Access to Leader Cluster](#set-up-access-to-leader-cluster) as
well (don't forget replace all `east` to `north` when you repeat the steps).

Then create the `ClusterClaim` and `ClusterSet` CRs in cluster
`test-cluster-north` in the `kube-system` Namespace (where the member
Multi-cluster Controller runs):

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha2
kind: ClusterClaim
metadata:
  name: id.k8s.io
  namespace: kube-system
value: test-cluster-north
---
apiVersion: multicluster.crd.antrea.io/v1alpha2
kind: ClusterClaim
metadata:
  name: clusterset.k8s.io
  namespace: kube-system
value: test-clusterset
---
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ClusterSet
metadata:
  name: test-clusterset
  namespace: kube-system
spec:
  leaders:
    - clusterID: test-cluster-north
      secret: "member-north-token"
      server: "https://172.18.0.1:6443"
  namespace: antrea-multicluster
```

## Multi-cluster Gateway Configuration

Multi-cluster Gateways are required to support multi-cluster Service access
across member clusters. Each member cluster should have one Node be specified as
its Multi-cluster Gateway. Multi-cluster Service traffic is routed among clusters
through the tunnels between Gateways.

After a member cluster joins a ClusterSet, and the `Multicluster` feature is
enabled on `antrea-agent`, you can select one Node of the cluster to serve as
the Multi-cluster Gateway by adding an annotation:
`multicluster.antrea.io/gateway=true` to the K8s Node. For example, you can run
the following command to annotate Node `node-1` as the Multi-cluster Gateway:

```bash
$kubectl annotate node node-1 multicluster.antrea.io/gateway=true
```

Multi-cluster Controller in the member cluster will detect the Gateway Node, and
create a `Gateway` CR with the same name as the Node. You can check it with command:

```bash
$kubectl get gateway -n kube-system
NAME          GATEWAY IP     INTERNAL IP    AGE
node-1        10.17.27.55    10.17.27.55    10s
```

`internalIP` of the Gateway is used for the tunnels between the Gateway Node and
other Nodes in the local cluster, while `gatewayIP` is used for the tunnels to
remote Gateways of other member clusters. Multi-cluster Controller discovers the
IP addresses from the K8s Node resource of the Gateway Node. It will always use
`InternalIP` of the K8s Node as the Gateway's `internalIP`. For `gatewayIP`,
there are several possibilities:

* By default, the K8s Node's `InternalIP` is used as `gatewayIP` too.
* You can choose to use the K8s Node's `ExternalIP` as `gatewayIP`, by changing
the configuration option `gatewayIPPrecedence` to value: `external`, when
deploying the member Multi-cluster Controller. The configration option is
defined in ConfigMap `antrea-mc-controller-config-***` in `antrea-multicluster-member.yml`.
* When the Gateway Node has a separate IP for external communication or is
associated with a public IP (e.g. an Elastic IP on AWS), but the IP is not added
to the K8s Node, you can still choose to use the IP as `gatewayIP`, by adding an
annotation: `multicluster.antrea.io/gateway-ip=<ip-address>` to the K8s Node.

When selecting the Multi-cluster Gateway Node, you need to make sure the
resulted `gatewayIP` can be reached from the remote Gateways. You may need to
[configure firewall or security groups](../network-requirements.md) properly to
allow the tunnels between Gateway Nodes. As of now, only IPv4 Gateway IPs are
supported.

After the Gateway is detected, Multi-cluster Controller will be responsible
for exporting the cluster's network information to other member clusters
through the leader cluster, including the cluster's Gateway IP and Service
CIDR. Multi-cluster Controller will try to discover the cluster's Service CIDR
automatically, but you can also manually specify the `serviceCIDR` option in
ConfigMap `antrea-mc-controller-config-***`. In other member clusters, a
ClusterInfoImport CR will be created for the cluster which includes the
exported network information. For example, in cluster `test-cluster-west`, you
you can see a ClusterInfoImport CR with name `test-cluster-east-clusterinfo`
is created for cluster `test-cluster-east`:

```bash
$kubectl get clusterinfoimport -n kube-system
NAME                            CLUSTER ID          SERVICE CIDR   AGE
test-cluster-east-clusterinfo   test-cluster-east   110.96.0.0/20  10s
```

Make sure you repeat the same step to assign a Gateway Node in all member
clusters. Once you confirm that all `Gateway` and `ClusterInfoImport` are
created correctly, you can follow the [Multi-cluster Service](#multi-cluster-service)
section to create multi-cluster Services and verify cross-cluster Service
access.

## Multi-cluster Service

After you set up a ClusterSet properly, you can create a `ServiceExport` CR to
export a Service from one cluster to other clusters in the Clusterset, like the
example below:

```yaml
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ServiceExport
metadata:
  name: nginx
  namespace: default
```

For example, once you export the `default/nginx` Service in member cluster
`test-cluster-west`, it will be automatically imported in member cluster
`test-cluster-east`. A Service and an Endpoints with name
`default/antrea-mc-nginx` will be created in `test-cluster-east`, as well as
a ServcieImport CR with name `default/nginx`. Now, Pods in `test-cluster-east`
can access the imported Service using its ClusterIP, and the requests will be
routed to the backend `nginx` Pods in `test-cluster-west`. You can check the
imported Service and ServiceImport with commands:

```bash
$kubectl get serviceimport antrea-mc-nginx -n default
NAME            TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)   AGE
antrea-mc-nginx ClusterIP   10.107.57.62 <none>        443/TCP   10s

$kubectl get serviceimport nginx -n default
NAME      TYPE           IP                AGE
nginx     ClusterSetIP   ["10.19.57.62"]   10s
```

As part of the Service export/import process, in the leader cluster, two
ResourceExport CRs will be created in the Multi-cluster Controller Namespace,
for the exported Service and Endpoints respectively, as well as two
ResourceImport CRs. You can check them in the leader cluster with commands:

```bash
$kubectl get resourceexport -n antrea-multicluster
NAME                                         CLUSTER ID          KIND          NAMESPACE     NAME      AGE
test-cluster-west-default-nginx-endpoints    test-cluster-west   Endpoints     default       nginx     30s
test-cluster-west-default-nginx-service      test-cluster-west   Service       default       nginx     30s

$kubectl get resourceimport -n antrea-multicluster
NAME                      KIND            NAMESPACE       NAME           AGE
default-nginx-endpoints   Endpoints       default         nginx          99s
default-nginx-service     ServiceImport   default         nginx          99s
```

When there is any new change on the exported Service, the imported multi-cluster
Service resources will be updated accordingly. Multiple member clusters can
export the same Service (with the same name and Namespace). In this case, the
imported Service in a member cluster will include endpoints from all the export
clusters, and the Service requests will be load-balanced to all these clusters.
Even when the client Pod's cluster also exported the Service, the Service
requests may be routed to other clusters, and the endpoints from the local
cluster do not take precedence. A Service cannot have conflicted definitions in
different export clusters, otherwise only the first export will be replicated to
other clusters; other exports as well as new updates to the Service will be
ingored, until user fixes the conflicts. For example, after a member cluster
exported a Service: `default/nginx` with TCP Port `80`, other clusters can only
export the same Service with the same Ports definition including Port names. At
the moment, Antrea Multi-cluster supports only IPv4 multi-cluster Services.

## Multi-cluster ClusterNetworkPolicy Replication

Since Antrea v1.6.0, Multi-cluster admins can specify certain
ClusterNetworkPolicies to be replicated and enforced across the entire
ClusterSet. This is especially useful for ClusterSet admins who want all
clusters in the ClusterSet to be applied with a consistent security posture (for
example, all Namespaces in all clusters can only communicate with Pods in their
own Namespaces). For more information regarding Antrea ClusterNetworkPolicy
(ACNP), please refer to [this document](../antrea-network-policy.md).

To achieve such ACNP replication across clusters, admins can, in the leader
cluster of a ClusterSet, create a `ResourceExport` CR of kind
`AntreaClusterNetworkPolicy` which contains the ClusterNetworkPolicy spec
they wish to be replicated. The `ResourceExport` should be created in the
Namespace where the ClusterSet's leader Multi-cluster Controller runs.

```yaml
apiVersion: multicluster.crd.antrea.io/v1alpha1
kind: ResourceExport
metadata:
  name: strict-namespace-isolation-for-test-clusterset
  namespace: antrea-multicluster # Namespace that Multi-cluster Controller is deployed
spec:
  kind: AntreaClusterNetworkPolicy
  name: strict-namespace-isolation # In each importing cluster, an ACNP of name antrea-mc-strict-namespace-isolation will be created with the spec below
  clusterNetworkPolicy:
    priority: 1
    tier: securityops
    appliedTo:
      - namespaceSelector: {} # Selects all Namespaces in the member cluster
    ingress:
      - action: Pass
        from:
          - namespaces:
              match: Self # Skip drop rule for traffic from Pods in the same Namespace
          - podSelector:
              matchLabels:
                k8s-app: kube-dns # Skip drop rule for traffic from the core-dns components
      - action: Drop
        from:
          - namespaceSelector: {} # Drop from Pods from all other Namespaces
```

The above sample spec will create an ACNP in each member cluster which
implements strict Namespace isolation for that cluster.

Note that because the Tier that an ACNP refers to must exist before the ACNP is applied, an importing
cluster may fail to create the ACNP to be replicated, if the Tier in the ResourceExport spec cannot be
found in that particular cluster. If there are such failures, the ACNP creation status of failed member
clusters will be reported back to the leader cluster as K8s Events, and can be checked by describing
the `ResourceImport` of the original `ResourceExport`:

```bash
$kubectl describe resourceimport -A
Name:         strict-namespace-isolation-antreaclusternetworkpolicy
Namespace:    antrea-multicluster
API Version:  multicluster.crd.antrea.io/v1alpha1
Kind:         ResourceImport
Spec:
  Clusternetworkpolicy:
    Applied To:
      Namespace Selector:
    Ingress:
      Action:          Pass
      Enable Logging:  false
      From:
        Namespaces:
          Match:  Self
        Pod Selector:
          Match Labels:
            k8s-app:   kube-dns
      Action:          Drop
      Enable Logging:  false
      From:
        Namespace Selector:
    Priority:  1
    Tier:      random
  Kind:        AntreaClusterNetworkPolicy
  Name:        strict-namespace-isolation
  ...
Events:
  Type    Reason               Age    From                       Message
  ----    ------               ----   ----                       -------
  Warning ACNPImportFailed     2m11s  resourceimport-controller  ACNP Tier random does not exist in the importing cluster test-cluster-west
```

In future releases, some additional tooling may become available to automate the
creation of ResourceExports for ACNPs, and provide a user-friendly way to define
Multi-cluster NetworkPolicies to be enforced in the ClusterSet.

## Build Antrea Multi-cluster Image

If you'd like to build Antrea Multi-cluster Docker image locally, you can follow
the following steps:

1. Go to your local `antrea` source tree, run `make antrea-mc-controller`, and you will get a new image
   named `antrea/antrea-mc-controller:latest` locally.
2. Run `docker save antrea/antrea-mc-controller:latest > antrea-mcs.tar` to save the image.
3. Copy the image file `antrea-mcs.tar` to the Nodes of your local cluster.
4. Run `docker load < antrea-mcs.tar` in each Node of your local cluster.

## Known Issue

We recommend user to redeploy or update Antrea Multi-cluster Controller through
`kubectl apply`. If you are using `kubectl delete -f *` and `kubectl create -f *`
to redeploy Controller in the leader cluster, you might encounter [a known issue](https://github.com/kubernetes/kubernetes/issues/60538)
in `ResourceExport` CRD cleanup. To avoid this issue, please delete any
`ResourceExport` CRs in the leader cluster first, and make sure
`kubectl get resourceexport -A` returns empty result before you can redeploy
Multi-cluster Controller.

All `ResourceExports` can be deleted with the following command:

```bash
kubectl get resourceexport -A -o json | jq -r '.items[]|[.metadata.namespace,.metadata.name]|join(" ")' | xargs -n2 bash -c 'kubectl delete -n $0 resourceexport/$1'
```
