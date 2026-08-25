# Security policy

## Scope

`storage-plugins` has two separate security surfaces: the offline planner in
the repository root and the privileged compatibility runtime under
`runtime/nfs`.

The planner accepts untrusted JSON and returns offline validation or planning
output. Its binary is intentionally non-operational: it has no network, cloud,
host, container-socket, credential, mount, format, delete, or command-execution
path. Every plan remains blocked even when an irreversible action is
acknowledged. A recursive AST policy reviews every planner production Go file,
allowlists imports, and restricts the CLI's `os` usage to its five required
process and stream selectors.

The runtime is operational and privileged by design. It connects only to the
configured control-plane origin, uses bounded HTTP responses and timeouts,
refuses cross-origin schema, collection, resource, and action links before
credentials can be sent, validates volume names and mount options, defaults to
retaining data, and limits `purge` to one validated driver-owned subdirectory.
Its Go module, vendored source, and three digest-pinned runtime images are built
and scanned separately from the planner.

## Reporting a vulnerability

Use the repository's private security-advisory workflow. Include the affected revision, a minimal redacted request, the observed result, and the expected safety property. Do not include credentials, secrets, private keys, internal addresses, raw host paths, or production identifiers; replace them with opaque `ref:` values.

Please avoid public issue disclosure until maintainers have acknowledged the report and coordinated a fix. Reports for delegated components belong to those components' own security processes.

## Validation expectations

Planner changes should preserve strict single-document JSON decoding, rejection
of every JSON `null` value, the 2 MiB input limit, the maximum nesting depth,
exact field casing, opaque external references, ownership and generation
checks, deterministic output, disabled controls, and blocked execution.
Optional fields must be absent rather than `null`. Runtime changes should
preserve same-origin credential handling, bounded network reads, retain-by-
default behavior, path containment, and exact dependency/image pins. Run the
validation that matches the changed surface before proposing a release.
