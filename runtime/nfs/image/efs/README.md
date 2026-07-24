# Amazon EFS storage driver

`pasturestack-efs` is a host-global external volume driver for compatible
PastureStack control planes. The maintained image is:

```text
ghcr.io/pasturestack/efs-storage-driver:<semantic-version>
```

Existing EFS filesystems are supported by default. Set
`ALLOW_CLOUD_PROVISIONING=true` only when the driver is authorized to create
and delete filesystems and mount targets. Provisioning also requires explicit
`EFS_SUBNET_ID` and `EFS_SECURITY_GROUP_ID` values. The driver never creates a
security group and never opens NFS access to an unrestricted network.

The driver uses EC2 IMDSv2 only when an AWS region was not supplied. IAM
instance profiles are preferred. The global service performs no cloud request
during initialization, so an unused driver remains healthy on non-AWS hosts.

The runtime requires privileged mode, host networking, the Docker socket,
`/run`, `/dev`, and the shared compatibility volume directory. It must run only
on trusted managed hosts.
