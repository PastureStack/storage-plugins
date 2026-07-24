# Compatibility and delegated components

`storage-plugins` defines new offline contracts under the PastureStack
namespace. The planner does not publish compatibility aliases or execute a
storage operation.

## Contract matrix

| Driver | Attachment model | Contract notes |
| --- | --- | --- |
| `aliyun-block` | device | Block size, disk category, and optional snapshot reference |
| `aws-ebs` | device | Block size, volume type, optional IOPS and snapshot reference, and optional opaque KMS reference |
| `aws-efs` | logical | Performance mode, export reference, and allowlisted mount options |
| `ceph-rbd` | device | Pool, block size, and one allowlisted image feature |
| `longhorn` | device | Byte size and replica count |
| `loop` | device | Development-only MiB-sized planning contract; mount operations are unsupported |
| `nfs` | logical | Server and export references, allowlisted mount options, and retain or owned-subdirectory purge policy |

Each supported operation models one adjacent lifecycle transition. A successful validation or plan means the request is structurally and semantically consistent; it does not establish runtime availability, API compatibility, data-plane compatibility, or execution readiness.

## NFS runtime compatibility boundary

The separately built `runtime/nfs` adapter provides the `pasturestack-nfs`
Docker volume driver to the preserved control-plane protocol. The public image
name, executable names, driver name, UI text, and Catalog entry use the
PastureStack namespace.

A small set of inherited protocol literals must remain unchanged so the adapter
can communicate with the preserved server and node agent:

- `CATTLE_URL`, `CATTLE_ACCESS_KEY`, and `CATTLE_SECRET_KEY`;
- the `/var/lib/rancher/volumes` and `/var/run/rancher/storage` host paths;
- the `rancher` managed-volume marker carried in the legacy API payload; and
- upstream Go package paths and exported type names inside vendored clients.

These values are compatibility identifiers, not branding claims. They are not
used as a repository name, image name, driver name, Catalog title, logo, or
visible product label.

The runtime has these destructive-operation constraints:

- `retain` is the default removal policy;
- a directly supplied export is always retained, even if `purge` is requested;
- `purge` can remove only one validated volume-name subdirectory under an
  export base; and
- traversal components, control characters, unsafe names, and unsafe mount
  options are rejected before a mount or removal command is attempted.

## Delegated secret delivery

Secret-delivery capabilities are delegated to separate repositories and are not included in this module, its request schema, or its binary:

- `PastureStack/secrets-flexvolume-plugin`
- `PastureStack/vault-secrets-bridge`

The public capability response reports both repositories with `included: false`. Their security boundaries, schemas, implementations, and release decisions remain independent from `storage-plugins`.
