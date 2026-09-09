# YAML Compatibility

Canonical agent-evaluation descriptors are JSON and must conform to
`HARNESS/schemas/agent-eval-case.schema.json`. A project may author YAML only
when its committed validator converts it deterministically to the canonical
JSON contract before validation. Do not maintain parallel JSON and YAML copies
of the same case.
