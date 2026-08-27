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

# Architecture Document Quality Gate

Use this checklist before marking a draft complete or finishing a review. Treat items marked **Required** as blockers when they apply. If the available evidence cannot establish a required item, report it as unresolved instead of passing it.

## Purpose and narrative

- [ ] **Required:** The opening states the core question or decision and why it matters.
- [ ] **Required:** The intended audience and scope can be inferred without reading the entire document.
- [ ] The document provides a useful answer at more than one reading depth.
- [ ] Central concepts are explained through intuition, scenario, mechanism, and boundary where needed.
- [ ] A concrete example is used when abstraction would otherwise impede understanding.
- [ ] Sections advance the core question or decision instead of becoming a component inventory.

## Technical integrity

- [ ] **Required:** Current implementation is separated from planned, exploratory, deprecated, and historical material, using the repository's equivalent labels when they exist.
- [ ] **Required:** Important implementation claims were verified against appropriate evidence or explicitly marked unverified.
- [ ] **Required:** Responsibilities and non-responsibilities are clear.
- [ ] **Required:** Meaningful failure paths, interruption, and recovery behavior are covered.
- [ ] Relevant states, transitions, invariants, ordering, and idempotency rules are stated.
- [ ] Important design choices include alternatives, costs, and revisit conditions.
- [ ] Code, interface, schema, test, or observability anchors are provided at the depth the document promises.
- [ ] Architecture-significant security, trust, privacy, performance, scalability, availability, compatibility, or cost constraints are addressed when relevant.

## Diagrams and examples

- [ ] Every diagram answers a specific question.
- [ ] Diagram terminology matches prose and implementation terminology.
- [ ] Reading direction and important omissions are explained.
- [ ] Examples do not imply guarantees that the system does not provide.

## Translation parity, when counterparts are in scope

- [ ] **Required:** All counterparts preserve scope, lifecycle status, certainty, guarantees, and limitations.
- [ ] **Required:** Canonical terms and code identifiers are consistent.
- [ ] Numbers, limits, states, ordering, examples, and failure behavior match.
- [ ] Each counterpart reads naturally in its own language.
- [ ] Shared diagrams and links work from every counterpart.
- [ ] Counterpart metadata or translation status is accurate when used.

## Maintainability

- [ ] The document follows existing repository layout and metadata conventions.
- [ ] Ownership and verification date are recorded when supported.
- [ ] Fragile duplication and unstable line-number references are avoided.
- [ ] Details owned elsewhere are linked rather than copied.
- [ ] Known uncertainty or stale areas are visible to future maintainers.

## Completion report

When handing off a document, state:

- files created or changed;
- evidence used to verify current behavior;
- planned or uncertain claims that remain;
- whether language counterparts passed semantic parity review, when applicable;
- any checklist items intentionally left open and why.
