PastureStack is an independent community effort to preserve, audit, and modernize the Rancher 1.6 ecosystem. It is not affiliated with or endorsed by Rancher Labs or SUSE.

**Upstream:** [`rancher/storage`](https://github.com/rancher/storage). This GitHub fork retains the upstream Git history, authorship, dates, and license notices unchanged; PastureStack maintenance is consolidated into one commit after the preserved upstream boundary.

# storage-plugins

`storage-plugins` is an alpha, offline contract validator and deterministic lifecycle planner for storage integrations. It preserves capability and state-transition intent without connecting to storage systems or changing host or cloud state.

This repository does not contain a runtime storage driver. It produces validation results and blocked `would-*` plans only.

## Driver contracts

The current contract surface contains exactly seven driver identifiers:

- `aliyun-block`
- `aws-ebs`
- `aws-efs`
- `ceph-rbd`
- `longhorn`
- `loop` (`developmentOnly: true`)
- `nfs`

The identifiers are new contracts, not compatibility aliases. Driver-specific fields, operation support, and delegated components are documented in [COMPATIBILITY.md](COMPATIBILITY.md).

## Safety boundary

The planner never executes a lifecycle action. Every generated step has `status: "blocked"`, every plan has `executable: false` and `effect: "none"`, and every runtime control is reported as `false`:

- network and cloud APIs
- Docker and host sockets
- host paths and privileged access
- mount, unmount, format, and delete
- credential and secret reads
- state writes and execution

An irreversible-action acknowledgement changes diagnostic context only; it never enables execution. See [SECURITY.md](SECURITY.md) for the threat model and reporting guidance.

A module-root AST gate recursively checks every production Go file, including files added under new package directories. Production imports are restricted to the reviewed standard-library set and module-local packages. The CLI entry point is the only production file allowed to import `os`, and only `Args`, `Stdin`, `Stdout`, `Stderr`, and `Exit` selectors are permitted.

## CLI

The binary exposes exactly three subcommands:

```text
storage-plugins capabilities --locale en-US
storage-plugins validate --locale en-US
storage-plugins plan --locale zh-TW
```

`capabilities` does not read standard input. `validate` and `plan` read exactly one JSON document from standard input, with a maximum size of 2 MiB. Supported locales are exactly `en-US` and `zh-TW`; the explicit message catalogs are in [`locales/`](locales/).

The decoder rejects duplicate, missing, unknown, or case-variant keys; multiple JSON documents; nesting deeper than 16 levels; invalid UTF-8; control characters; unsafe identifiers; raw paths or URLs; arbitrary mount-option strings; and secret or credential material. JSON `null` is rejected at every depth and for every field type; optional fields must be omitted instead.

## Request contract

Requests use `storage-plugins.pasturestack.io/v1alpha1` and `LifecyclePlanRequest`. The lifecycle is:

```text
absent -> available -> attached -> mounted
absent <- available <- attached <- mounted
```

The operation selects one transition: `create`, `attach`, `mount`, `unmount`, `detach`, or `remove`. Ownership, observed generation, expected generation, and an idempotency key are required. External systems are represented only by safe opaque `ref:` or `sha256:` references.

Ready-to-run NFS and block-storage requests, plus commands for all three subcommands, are in [`examples/`](examples/).

Plans have a deterministic `sha256:` identifier derived from normalized semantic input. They contain sorted gates, blocked steps, diagnostics, and the complete disabled-control object. They contain no clock, random, credential, or secret data.

## Build and test

The core uses Go 1.26 and only the standard library:

```sh
go test ./...
go vet ./...
go mod verify
go build -trimpath -buildvcs=false ./cmd/storage-plugins
```

The repository validation scripts add public-tree, legal-file, reproducibility, binary-content, and example smoke gates:

```text
pwsh -File scripts/validate.ps1
sh scripts/validate.sh
```

The test suite covers all seven driver contracts, strict JSON behavior, the lifecycle table, ownership and generation gates, deterministic idempotency, irreversible-action blocking, locale parity, and forbidden dangerous imports or calls.

## License and provenance

The inherited root [`LICENSE`](LICENSE) remains unchanged. Go toolchain notices are kept separately in [`LICENSES/`](LICENSES/). See [ORIGIN.md](ORIGIN.md) for the source, deletion, and distribution boundaries.
