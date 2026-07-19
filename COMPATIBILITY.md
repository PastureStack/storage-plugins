# Compatibility and delegated components

`storage-plugins` defines new offline contracts under the PastureStack namespace. It does not publish compatibility aliases and does not execute an existing storage driver.

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

## Delegated secret delivery

Secret-delivery capabilities are delegated to separate repositories and are not included in this module, its request schema, or its binary:

- `PastureStack/secrets-flexvolume-plugin`
- `PastureStack/vault-secrets-bridge`

The public capability response reports both repositories with `included: false`. Their security boundaries, schemas, implementations, and release decisions remain independent from `storage-plugins`.
