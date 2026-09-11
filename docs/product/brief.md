# Product brief

Concept v0.2 · September 2026 · Architecture and product concept

## Problem

Documents, metadata, and audit logs often share a single administrative trust boundary. Privileged changes may be hard to distinguish from authentic history. SigmaProof creates externally committed evidence so verification need not depend solely on the operational database.

## Product

SigmaProof combines an evidence protocol, policy engine, and independent verification tools. It records document identity and integrity, metadata commitments, versions, lifecycle events, provenance assertions, external observations, signatures, retention evidence, and renewal. It does not replace document management or preservation systems.

The initial user is an organization operating Paperless-ngx that needs to export independently verifiable evidence. Later integrations include S3-compatible storage, Nextcloud, SharePoint, Microsoft 365, Google Workspace, ERP/accounting platforms, and custom DMS applications. Future evidence subjects include datasets, audit logs, software artifacts, container images, and AI-generated records.

## User journeys

1. An operator selects an evidence policy and connects a source system.
2. An agent identifies a specific document version and representation, hashes its bytes, and submits private evidence.
3. The engine batches commitments, captures anchor proofs, and exports a package back to customer-controlled storage.
4. A recipient verifies the document and package independently, with explicit outcomes for each assertion and policy requirement.
5. A preservation operator renews evidence before cryptographic or trust assumptions become unsuitable.

Paperless should show evidence ID, document version, policy, verification status, anchor status, and links to verify, export, and inspect history. UI controls are proposed; stock Paperless custom fields alone do not implement a new document panel.

## Guarantees and limits

Evidence can demonstrate consistency with committed bytes and independently authenticated anchor observations. Claimed event time differs from externally observed time. Provenance is an assertion whose credibility depends on identity, signatures, source controls, and policy.

SigmaProof does not inherently prove truthfulness, upload authorization, legal signature validity, qualified timestamps, complete history, regulatory retention compliance, or DMS compliance. Policy satisfaction is evaluated against a particular preserved policy, not universal legal adequacy.

## Positioning

**SigmaProof provides independently verifiable evidence for digital documents.**

Enterprise: independent evidence infrastructure for document integrity, provenance, and compliance workflows.

Technical: an open evidence protocol with portable proofs and policy-driven multi-anchor verification.

OpenTimestamps is established prior art and a prospective provider. SigmaProof adds lifecycle modeling, policy evaluation, and provider diversity; it does not claim to invent blockchain timestamping.

## Success measures

The first release must demonstrate verification after anchor blob pruning, customer-controlled proof export, corruption detection, and no identifying data in public payloads. Track proof size, batch cost, anchoring latency, capture failures, restore success, and independently reproduced verification. Performance targets follow measured prototypes.
