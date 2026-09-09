# Development Plan

## DDAE-1 Deliver a Go DDAE monitoring exporter with Prometheus and Kafka outputs

### Approval Record

- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Peter Chu
- Requirements Confirmer Role: Project Owner / Requirements Approver
- Requirements Confirmed On: 2026-08-24
- Requirements Confirmation Evidence: Peter Chu explicitly confirmed Specification v1.0.0 and digest sha256:27806c9f0dbd8dae57c818bc40839d5ae0983ed2d4cd2937b5a98489eacdf40a in the Codex conversation on 2026-08-24
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Peter Chu
- Implementation Plan Approver Role: Project Owner / Implementation Plan Approver
- Implementation Plan Approved On: 2026-08-24
- Implementation Plan Approval Evidence: Peter Chu explicitly approved plan-basis digest sha256:470ca282d75c846e09b9ad4fddcb872d8377fbb998afbc85eb7e6416e19d1a4d and test-plan digest sha256:fbed959f4495bb1f0eab56fb3922285e2c7e450a93ecdf3392de387743fa452e in the Codex conversation on 2026-08-24 and authorized implementation
- Approved Specification Version: 1.0.0
- Approved Specification Digest: sha256:27806c9f0dbd8dae57c818bc40839d5ae0983ed2d4cd2937b5a98489eacdf40a
- Approved Plan Basis Digest: sha256:470ca282d75c846e09b9ad4fddcb872d8377fbb998afbc85eb7e6416e19d1a4d
- Approved Test Plan Digest: sha256:fbed959f4495bb1f0eab56fb3922285e2c7e450a93ecdf3392de387743fa452e

`Peter Chu` is the named actor for the project-owner, approval, security/DDAE,
Prometheus/SRE, Kafka/OpenSearch and operational-owner roles. The digest-bound
requirements confirmation and implementation-plan approval are recorded above.

Environment-dependent DDAE, Kafka and OpenSearch validation is deferred because
the required environment is not currently available. No validation result is
claimed. The deferred work remains a release blocker and may require renewed
approval if live structural evidence conflicts with the approved v1 contract.

`Implementation Plan Approved` covers both the marker-bounded plan basis and
the test-plan digest above. If either changes, renew this named approval.
Changing the active specification also invalidates Requirements Confirmed and
its approved specification version and digest. When both sources change, renew
both named approvals. Never replace a digest while retaining the prior actor,
date, and evidence as though they approved the new bytes.

### Execution Plan Reference

- Execution Plan: plans/DDAE-1.md

## DDAE-2 Add YAML configuration, independent monitoring pipelines and guarded TLS controls

### Approval Record

- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Peter Chu
- Requirements Confirmer Role: Project Owner / Requirements Approver
- Requirements Confirmed On: 2026-08-25
- Requirements Confirmation Evidence: Peter Chu explicitly confirmed DDAE-2 Specification v1.1.0 and digest sha256:257851ec53342f7e4898e99ca19d29fe759e7853add4cf4826704b9179bfe9f9 in the Codex conversation on 2026-08-25
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Peter Chu
- Implementation Plan Approver Role: Project Owner / Implementation Plan Approver
- Implementation Plan Approved On: 2026-08-25
- Implementation Plan Approval Evidence: Peter Chu explicitly approved DDAE-2 plan-basis digest sha256:904cd361cb70b6bdc3aa38ddf5613fd0b65ecf52acaac6b76ecc7ff8aa2a010c and test-plan digest sha256:104e6e7ca43fd2f2dfe9a2a84581011f63878859da4c4c19c86befe0dfa73917 in the Codex conversation on 2026-08-25 and authorized implementation
- Approved Specification Version: 1.1.0
- Approved Specification Digest: sha256:257851ec53342f7e4898e99ca19d29fe759e7853add4cf4826704b9179bfe9f9
- Approved Plan Basis Digest: sha256:904cd361cb70b6bdc3aa38ddf5613fd0b65ecf52acaac6b76ecc7ff8aa2a010c
- Approved Test Plan Digest: sha256:104e6e7ca43fd2f2dfe9a2a84581011f63878859da4c4c19c86befe0dfa73917

`Peter Chu` is the named actor for the project-owner, requirements,
implementation-plan, security/DDAE, Prometheus/SRE, Kafka/OpenSearch and
operational-owner roles. The digest-bound requirements confirmation and
implementation-plan approval are recorded above.

DDAE-2 depends on DDAE-1 and does not waive its external validation,
supply-chain, security or independent-review blockers. Insecure-TLS runs are
diagnostic only and cannot provide release or production-support evidence.

The approved Specification, marker-bounded Plan Basis and Test Plan remain
digest locked. Any governed-byte change requires refreshed digests and renewed
named confirmation/approval before further affected implementation.

### Execution Plan Reference

- Execution Plan: plans/DDAE-2.md

## DDAE-3 Harden delivery correctness, bounded failure and state health

### Approval Record

- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Peter Chu
- Requirements Confirmer Role: Project Owner / Requirements Approver
- Requirements Confirmed On: 2026-08-26
- Requirements Confirmation Evidence: Peter Chu explicitly confirmed DDAE-3 Specification v1.2.0 and digest sha256:5560a767e722c6649b98f9b57e4e2c98bfce5331559f67b07ffbf729811e18f7 in the Codex conversation on 2026-08-26
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Peter Chu
- Implementation Plan Approver Role: Project Owner / Implementation Plan Approver
- Implementation Plan Approved On: 2026-08-26
- Implementation Plan Approval Evidence: Peter Chu explicitly approved DDAE-3 plan-basis digest sha256:fa4fc25743fe0cb55f5097ed1edf440f4a8fdf66803869861a189ad47022e184 and test-plan digest sha256:22ea52ce6525bf5a4447e885d6da50072a9dde1f514e9d9e148e20da4bb74a80 in the Codex conversation on 2026-08-26 and authorized Batch 1 implementation
- Approved Specification Version: 1.2.0
- Approved Specification Digest: sha256:5560a767e722c6649b98f9b57e4e2c98bfce5331559f67b07ffbf729811e18f7
- Approved Plan Basis Digest: sha256:fa4fc25743fe0cb55f5097ed1edf440f4a8fdf66803869861a189ad47022e184
- Approved Test Plan Digest: sha256:22ea52ce6525bf5a4447e885d6da50072a9dde1f514e9d9e148e20da4bb74a80

Peter Chu is the named actor for the project-owner, requirements and
implementation-plan approval roles. The confirmation covers the exact
Specification digest above; implementation authorization currently covers
Batch 1 under the exact Plan Basis and Test Plan digests above. Batch 2 remains
ordered after Batch 1's mapped local evidence, as defined by the same approved
basis.

`Implementation Plan Approved` covers both the marker-bounded plan basis and
the test-plan digest above. If either changes, renew this named approval.
Changing the active specification also invalidates Requirements Confirmed and
its approved specification version and digest. Never replace a digest while
retaining prior approval evidence as though it approved new bytes.

### Execution Plan Reference

- Execution Plan: plans/DDAE-3.md

## DDAE-4 Add Serviceability Logs / Events collection and Kafka delivery

### Approval Record

- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Peter Chu
- Requirements Confirmer Role: Project Owner / Requirements Approver
- Requirements Confirmed On: 2026-08-28
- Requirements Confirmation Evidence: Peter Chu explicitly confirmed DDAE-4 Specification v1.3.0 and digest sha256:b39afeaebb877ad4af49ab56f052032391e6af9251bbb26023d1c84babb9cbd7 in the Codex conversation on 2026-08-28
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Peter Chu
- Implementation Plan Approver Role: Project Owner / Implementation Plan Approver
- Implementation Plan Approved On: 2026-08-28
- Implementation Plan Approval Evidence: Peter Chu explicitly approved DDAE-4 Plan Basis digest sha256:ec83f178c0824ba85ed3ffacdea92fb4353e578c7e26a8ecd2890c42df629111 and Test Plan digest sha256:1986c9a5f345cef6933f29750f3951911787cd77e58955c09216190e35fc92e7, confirmed module path github.com/crispkid/dell-ddae-metrics-exporter, accepted the implemented DDAE-3 Batch 2 local baseline, preserved external release gates and authorized one-pass local implementation and unit/component testing in the Codex conversation on 2026-08-28
- Approved Specification Version: 1.3.0
- Approved Specification Digest: sha256:b39afeaebb877ad4af49ab56f052032391e6af9251bbb26023d1c84babb9cbd7
- Approved Plan Basis Digest: sha256:ec83f178c0824ba85ed3ffacdea92fb4353e578c7e26a8ecd2890c42df629111
- Approved Test Plan Digest: sha256:1986c9a5f345cef6933f29750f3951911787cd77e58955c09216190e35fc92e7

Peter Chu is the named actor for the project-owner, requirements and
implementation-plan approval roles. The approval package records the user's
direction to complete DDAE-4 implementation and local unit/component tests in
one development pass while excluding unavailable surrounding-environment
validation from that pass.

Approval of the exact Plan Basis explicitly accepts the implemented DDAE-3
Batch 2 state-integrity, readiness, token, fairness and HTTP-bound behavior as
the DDAE-4 implementation baseline. This does not mark DDAE-3 `Verified` and
does not waive DDAE, Kafka, OpenSearch, deployment, independent-review,
supply-chain or inherited release gates.

`Implementation Plan Approved` covers both the marker-bounded Plan Basis and
the Test Plan digest. Product code may not change for DDAE-4 until Peter Chu
confirms the exact Specification digest and approves both exact plan digests.

### Execution Plan Reference

- Execution Plan: plans/DDAE-4.md

## DDAE-5 Add independently configurable DDAE API path prefixes

### Approval Record

- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Peter Chu
- Requirements Confirmer Role: Project Owner / Requirements Approver
- Requirements Confirmed On: 2026-08-31
- Requirements Confirmation Evidence: Peter Chu explicitly confirmed DDAE-5 Specification v2.0.0 and digest sha256:636bf0be364097005714d3062e164b981a0a139cbc0e1e00d93cbdc8e753ce35 in the Codex conversation on 2026-08-31
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Peter Chu
- Implementation Plan Approver Role: Project Owner / Implementation Plan Approver
- Implementation Plan Approved On: 2026-08-31
- Implementation Plan Approval Evidence: Peter Chu explicitly approved DDAE-5 Plan Basis digest sha256:ff5809908ea3990e7f1682a0f76e222c7cd19eceb80040deda82b1e8557c51b8 and Test Plan digest sha256:27fc6f5fd370441404395b8df6ea16b6ab290789ebec56b98c17253470153298, confirmed module path github.com/crispkid/dell-ddae-metrics-exporter and authorized one-pass implementation with local unit, component and security testing while retaining authenticated DDAE integration and inherited external release gates in the Codex conversation on 2026-08-31
- Approved Specification Version: 2.0.0
- Approved Specification Digest: sha256:636bf0be364097005714d3062e164b981a0a139cbc0e1e00d93cbdc8e753ce35
- Approved Plan Basis Digest: sha256:ff5809908ea3990e7f1682a0f76e222c7cd19eceb80040deda82b1e8557c51b8
- Approved Test Plan Digest: sha256:27fc6f5fd370441404395b8df6ea16b6ab290789ebec56b98c17253470153298

Peter Chu selected the two-prefix product design and the `/ping` plus `/v1/*`
defaults on 2026-08-31, then confirmed and approved the exact Specification,
Plan Basis and Test Plan identities recorded above.

The proposed implementation scope covers product code, deterministic local
unit/component tests, security policy and documentation. Authorized
authenticated DDAE integration and every inherited external release gate
remain separate and cannot be satisfied with the supplied 401/404 observations
or local test doubles.

### Execution Plan Reference

- Execution Plan: plans/DDAE-5.md

## DDAE-6 Align DDAE 1.5.0 response decoding with documented and observed payloads

### Approval Record

- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Peter Chu
- Requirements Confirmer Role: Project Owner / Requirements Approver
- Requirements Confirmed On: 2026-09-02
- Requirements Confirmation Evidence: Peter Chu explicitly confirmed DDAE-6 Specification v2.1.0 and digest sha256:1385fb34bc0edb906d36f23600dcf528fa5eb7e2166fe57c80ea40a407a88f4b in the Codex conversation on 2026-09-02
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Peter Chu
- Implementation Plan Approver Role: Project Owner / Implementation Plan Approver
- Implementation Plan Approved On: 2026-09-02
- Implementation Plan Approval Evidence: Peter Chu explicitly approved DDAE-6 Plan Basis digest sha256:f42d5560e2680e7213c21da028293d3b33f85d1bcd944d0c0b2d6490ef25fd76 and Test Plan digest sha256:ce0bd41c98108aca7e7f5b355ca45a5578a179b53cdf7730e0cd43c4ac78fec8, confirmed module path github.com/crispkid/dell-ddae-metrics-exporter and authorized one-pass implementation with local unit, component and security testing while retaining authenticated DDAE integration and inherited external release gates in the Codex conversation on 2026-09-02
- Approved Specification Version: 2.1.0
- Approved Specification Digest: sha256:1385fb34bc0edb906d36f23600dcf528fa5eb7e2166fe57c80ea40a407a88f4b
- Approved Plan Basis Digest: sha256:f42d5560e2680e7213c21da028293d3b33f85d1bcd944d0c0b2d6490ef25fd76
- Approved Test Plan Digest: sha256:ce0bd41c98108aca7e7f5b355ca45a5578a179b53cdf7730e0cd43c4ac78fec8

Peter Chu confirmed and approved the exact DDAE-6 identities recorded above
and authorized the proposed local implementation pass. DDAE-6 uses a new
Change ID because response-shape compatibility is materially different from
DDAE-5 route-prefix behavior.

The proposed local pass covers synthetic fixtures, the bounded node decoder,
Informational severity normalization, serviceability list/detail regression,
security policy and concise operator documentation. It explicitly excludes raw
field payload retention, invented pagination, authenticated DDAE integration
and inherited external release gates.

`Implementation Plan Approved` covers the exact marker-bounded Plan Basis and
Test Plan digest above. Any governed-byte change requires refreshed digests and
renewed named approval before affected implementation continues.

### Execution Plan Reference

- Execution Plan: plans/DDAE-6.md

## DDAE-7 Windows 11 portable diagnostics and protected HTTP capture

### Approval Record

- Requirements Confirmed Status: confirmed
- Requirements Confirmed By: Peter Chu
- Requirements Confirmer Role: Peter Chu / Project Owner / Requirements Approver
- Requirements Confirmed On: 2026-09-07
- Requirements Confirmation Evidence: Peter Chu explicitly confirmed DDAE-7 Specification v2.2.0 digest sha256:a6d6162d5dd83a384ee12459a0c6332e9c862733a352b65e566bfa5c7d242676 in the current conversation, including encrypted original business API retention with authentication exclusions and separate private-key custody
- Implementation Plan Approved Status: approved
- Implementation Plan Approved By: Peter Chu
- Implementation Plan Approver Role: Peter Chu / Project Owner / Implementation Plan Approver
- Implementation Plan Approved On: 2026-09-07
- Implementation Plan Approval Evidence: Peter Chu explicitly approved Plan Basis sha256:895e67358ecaf79264b2686cf00f5401cafb083a473bf45e297e6a8b77cf2e32 and Test Plan sha256:d01fdc00aa0bd2d13282872bd6390375394ed7c8fb8f924f11fa429aebd90c92, authorized one-pass Portable implementation and local unit/component/security/cross-build tests, and retained native Windows, authenticated DDAE and external release gates
- Approved Specification Version: 2.2.0
- Approved Specification Digest: sha256:a6d6162d5dd83a384ee12459a0c6332e9c862733a352b65e566bfa5c7d242676
- Approved Plan Basis Digest: sha256:895e67358ecaf79264b2686cf00f5401cafb083a473bf45e297e6a8b77cf2e32
- Approved Test Plan Digest: sha256:d01fdc00aa0bd2d13282872bd6390375394ed7c8fb8f924f11fa429aebd90c92

Candidate identities calculated by `./HARNESS/harness.sh sdd:digest` on
2026-09-07 (these are not approval records):

- Candidate Specification Version: 2.2.0
- Candidate Specification Digest: sha256:a6d6162d5dd83a384ee12459a0c6332e9c862733a352b65e566bfa5c7d242676
- Candidate Plan Basis Digest: sha256:895e67358ecaf79264b2686cf00f5401cafb083a473bf45e297e6a8b77cf2e32
- Candidate Test Plan Digest: sha256:d01fdc00aa0bd2d13282872bd6390375394ed7c8fb8f924f11fa429aebd90c92

The candidate targets Specification v2.2.0. The proposed local pass delivers
Portable executables/templates/launchers, protected business HTTP capture,
safe reporting, key tools, offline modes and local unit/component/security/build
tests. Authentication bodies and deliberate credential retention remain
excluded. Peter Chu approved the explicit raw-data exception and digests above.

Native Windows, authorized non-production DDAE, independent review and inherited
external release gates remain separate. No release/tag/commit is authorized.
Historical DDAE-6 approval bytes and unrelated product edits are preserved.

### Execution Plan Reference

- Execution Plan: plans/DDAE-7.md
