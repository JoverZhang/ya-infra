---
doc_id: architecture.<subject>
title: "<Reader-oriented title>"
document_type: "<system-overview|component|mechanism|data-flow|deployment|decision>"
implementation_status: "<current|planned|exploratory|deprecated|historical>"
document_status: "<draft|stable|deprecated|historical>"
last_verified: "YYYY-MM-DD"
owners:
  - "<team-or-role>"
# For maintained translations, add language, source_language, counterpart,
# and translation_status fields when the repository has no equivalent convention.
---
<!--
  Licensed to the Apache Software Foundation (ASF) under one
  or more contributor license agreements.  See the NOTICE file
  distributed with this work for additional information
  regarding copyright ownership.  The ASF licenses this file
  to you under the Apache License, Version 2.0 (the
  "License"); you may not use this file except in compliance
  with the License.  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing,
  software distributed under the License is distributed on an
  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
  KIND, either express or implied.  See the License for the
  specific language governing permissions and limitations
  under the License.
-->

# <Reader-oriented title>

> In one paragraph: what question or decision this document addresses, the conclusion, and why it matters.

## Why this matters

Describe the engineering or reader problem. Define the scope, system context, and relevant exclusions.

## Mental model

Give the simplest correct intuition. Define central terms before relying on them.

## A concrete scenario

Introduce one representative example that can continue through the document.

## How it works

Explain the mechanism, sequence, data, and component responsibilities. Add only diagrams that answer a specific question. Adapt this section to the chosen artifact type.

## State and invariants

Describe lifecycle, durable and ephemeral state, valid transitions, ownership, ordering, and conditions that must remain true.

## Boundaries

State what this design owns, what it delegates, and what it explicitly does not guarantee. Include trust boundaries and architecture-significant quality constraints when relevant.

## Failure and recovery

Cover timeouts, interruption, cancellation, retry, partial success, recovery, and irrecoverable failure as applicable.

## Decisions and trade-offs

Explain the chosen design, credible alternatives, benefits, costs, and revisit conditions.

## Code and operational map

Point to stable modules, types, interfaces, schemas, tests, logs, traces, and metrics appropriate to the promised reading depth.

## Known limitations and future direction

Keep Current, Planned, and Exploratory claims visibly separate.

## Further reading

Link related architecture documents, ADRs, API specifications, and code-owned references.

<!-- Follow repository conventions and remove any section that does not help answer the document's core question. -->
