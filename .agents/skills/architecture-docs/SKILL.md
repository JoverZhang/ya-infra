---
name: architecture-docs
description: Draft, restructure, translate, or review evidence-backed architecture documentation. Use for system, component, mechanism, data-flow, deployment, or architecture-decision narratives that must explain boundaries, lifecycle status, failure behavior, and trade-offs. Do not use for API references, operational runbooks, or generic prose editing unless architectural reasoning is central.
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

# Architecture Docs

Create architecture documentation that is easy to enter, technically rigorous, and maintainable. Treat the user's subject, audience, scope, language, and chosen organization as constraints. This skill governs evidence, narrative quality, technical precision, translation parity when needed, and review.

## Select the task mode

- **Draft**: create a new document from code, design notes, interviews, or a user brief.
- **Rewrite**: improve an existing document while preserving its technical meaning.
- **Translate**: produce a natural counterpart in the target language while preserving technical meaning.
- **Review**: identify concrete gaps in clarity, evidence, correctness, or counterpart consistency when translations exist.
- **Standardize**: align multiple documents on terminology, metadata, status labels, and structure without forcing them into identical narratives.

Confirm the intended mode from the request. Infer it when safe; ask only when different interpretations would materially change the output.

## Follow the workflow

1. **Establish the document contract.** State the primary reader question or decision, intended reader, expected reading depth, scope, evidence basis, and claimed implementation status. Infer these when the request and repository make them clear. Preserve the user's outline unless changing it is part of the request.
2. **Choose the artifact shape.** Follow repository conventions for an overview, component guide, mechanism explanation, data or deployment view, ADR, or another established form. Let the question determine the structure.
3. **Collect evidence.** For current-state claims, inspect relevant code paths, tests, schemas, configuration, ADRs, runtime signals, and existing documents before presenting implementation details as facts. Match the evidence to the claim, and separate verified behavior, accepted decisions, plans, and interpretation.
4. **Build the narrative.** Move from problem and intuition to a representative scenario, mechanism, boundaries, failures, and trade-offs when those elements help answer the core question.
5. **Add technical anchors.** Connect claims to components, state transitions, invariants, interfaces, trust boundaries, operational signals, or stable code locations at the promised reading depth.
6. **Handle language counterparts when requested.** Preserve scope, status, certainty, examples, diagrams, and decision rationale while writing naturally for the target audience.
7. **Run the quality gate.** Review the artifact against `references/quality-gate.md`. For Draft, Rewrite, Translate, and Standardize work, fix failed required checks that apply or report them as unresolved when evidence is unavailable. For Review work, report the failures without changing files unless the user also asked for fixes.

Read `references/writing-standard.md` for Draft, Rewrite, Review, and Standardize work. Read `references/translation-standard.md` only when translating or comparing language counterparts. Use `assets/document-template.md` for a new narrative document only when the repository has no better scaffold and the template fits the subject; remove irrelevant sections rather than filling them mechanically.

## Apply non-negotiable rules

- Start from a reader question, architecture decision, or engineering problem, not a component inventory.
- Distinguish current, planned, exploratory, deprecated, and historical claims, using the repository's equivalent labels when they exist.
- Never present an unverified design assumption as current implementation.
- Explain important mechanisms through intuition, scenario, mechanism, and boundary.
- Cover meaningful failure paths and trade-offs, not only the happy path.
- Keep diagrams purposeful and language-neutral where practical; explain what each diagram includes and omits.
- Use stable terms. Do not alternate among synonyms for variety when they represent the same architectural concept.
- Cover material quality attributes and trust boundaries when they affect the design; omit boilerplate categories that do not.
- Preserve uncertainty across rewrites and translations. Do not strengthen or weaken a claim.
- Optimize for progressive depth: a reader should gain value from the opening summary without reading the entire document.

## Handle reviews

Lead with actionable findings ordered by impact. Cite the exact section or line when possible. Check, in order:

1. factual correctness and status confusion;
2. missing boundaries, invariants, or failure behavior;
3. broken narrative or unexplained concepts;
4. semantic drift and terminology inconsistency across language counterparts, when present;
5. maintainability issues such as duplicated diagrams or unstable code references;
6. style polish.

If there are no material findings, say so and identify any verification limits.

## Deliver artifacts

When creating files, follow the repository's existing documentation layout. If no convention exists, recommend rather than silently establish a repository-wide structure. For maintained translations, follow the project's counterpart naming or stable document-ID convention. Report which technical claims were verified, which evidence was used, and which claims remain planned, inferred, or uncertain.
