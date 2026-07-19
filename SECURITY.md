# Security policy

## Scope

`storage-plugins` accepts untrusted JSON and returns offline validation or planning output. The supported security surface is the decoder, contract validation, deterministic planner, embedded locale catalogs, and capability output in this repository.

The binary is intentionally non-operational. It has no network client, cloud API client, host or container socket access, host-path access, privileged mode, credential or secret reader, state writer, mount or format implementation, delete implementation, or command execution path. Every plan remains blocked even when an irreversible action is acknowledged. A recursive AST policy reviews every production Go file under the module root, allowlists imports, and restricts the CLI's `os` usage to its five required process and stream selectors.

## Reporting a vulnerability

Use the repository's private security-advisory workflow. Include the affected revision, a minimal redacted request, the observed result, and the expected safety property. Do not include credentials, secrets, private keys, internal addresses, raw host paths, or production identifiers; replace them with opaque `ref:` values.

Please avoid public issue disclosure until maintainers have acknowledged the report and coordinated a fix. Reports for delegated components belong to those components' own security processes.

## Validation expectations

Changes should preserve strict single-document JSON decoding, rejection of every JSON `null` value, the 2 MiB input limit, the maximum nesting depth, exact field casing, opaque external references, ownership and generation checks, deterministic output, disabled controls, and blocked execution. Optional fields must be absent rather than `null`. New production imports or broader operating-system selectors require an explicit policy review. Run the platform-appropriate script under [`scripts/`](scripts/) before proposing a release.
