# CLI examples

The two request fixtures contain only synthetic identifiers and opaque references:

- [`nfs-create.json`](nfs-create.json) exercises `validate` with a logical storage contract.
- [`aws-ebs-create.json`](aws-ebs-create.json) exercises `plan` with an irreversible filesystem-preparation intent that remains blocked.

Build the binary or substitute `go run ./cmd/storage-plugins` for `storage-plugins` in the commands below.

```sh
storage-plugins capabilities --locale en-US
storage-plugins validate --locale en-US < examples/nfs-create.json
storage-plugins plan --locale en-US < examples/aws-ebs-create.json
```

PowerShell can pipe the request content:

```powershell
storage-plugins capabilities --locale en-US
Get-Content -Raw examples/nfs-create.json | storage-plugins validate --locale en-US
Get-Content -Raw examples/aws-ebs-create.json | storage-plugins plan --locale en-US
```

The validation result must be valid while exposing only disabled controls. The plan must contain two blocked `would-*` steps, an irreversible gate that remains blocked, `executable: false`, and `effect: "none"`. Running the same plan command twice produces identical bytes.
