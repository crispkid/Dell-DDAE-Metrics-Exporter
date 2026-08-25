# Sanitized DDAE 1.5.0 fixtures

These files are deterministic synthetic inputs derived from the field contract
in Specification v1.0.0. They contain no captured DDAE response, credential,
private endpoint, production identifier or personal data. Reserved
`example.invalid` values and explicit `synthetic-*` strings exercise exclusion
boundaries without resembling acceptance evidence from a real appliance.

The fixtures cover ping, cluster configuration/status, node state/capacity,
lock and power readiness, alert list/detail, unknown sensitive alert fields and
malformed JSON. `alert-event.golden.json` is the exact schema-1.0 output for the
approved alert detail fixture at the fixed observation time documented in the
regression test.

Do not replace these files with raw environment captures. Authorized
integration evidence must be structurally sanitized before retention and is
governed separately from this fixture suite.
