# Amazon EBS storage driver

`pasturestack-ebs` is a host-global external volume driver for compatible
PastureStack control planes. The maintained image is:

```text
ghcr.io/pasturestack/ebs-storage-driver:<semantic-version>
```

The driver uses EC2 IMDSv2 when region, availability-zone, or instance
information is not supplied through the standard AWS environment. IAM instance
profiles are preferred. A matched access-key and secret-key pair remains
available for inherited deployments, but credentials must never be committed
to Catalog templates or source control.

Existing EBS volumes are supported by default. Set
`ALLOW_CLOUD_PROVISIONING=true` only when this driver is authorized to create,
format, detach, and delete EBS resources. New volumes are encrypted by default.
The opt-in must be reviewed together with the IAM policy and data-retention
requirements.

The runtime requires privileged mode, host networking, the Docker socket,
`/run`, `/dev`, and the shared compatibility volume directory. It must run only
on trusted managed hosts.
