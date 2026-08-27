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

# Architecture Documentation Translation Standard

## Publishing model

When a project maintains the same document in multiple languages, prefer separate, complete counterparts. Use the repository's matching-path, filename, or stable `doc_id` convention. Reuse diagrams, code snippets, schemas, and other language-neutral assets.

Use a side-by-side or interleaved format only when that format serves the requested reading or review workflow.

## Source and counterpart

Choose one source language for each editing cycle and establish which version is authoritative. When technical verification is in scope, verify the source before creating the counterpart. Otherwise preserve its degree of certainty and flag contradictions or suspected errors instead of silently repairing them. The counterpart is an adaptation for readers in the target language, not a sentence-by-sentence transliteration.

Every language counterpart must preserve:

- the core question and scope;
- technical claims and their lifecycle status;
- degree of certainty;
- examples and identifiers;
- diagram semantics;
- decision rationale and trade-offs;
- warnings, limitations, and failure behavior.

Natural differences in sentence structure, section phrasing, and explanatory context are allowed. Do not add a technical claim to only one counterpart without intentionally updating or marking the others.

## Terminology

For recurring project concepts, follow an existing glossary or maintain a small task-local term map with the fields that apply:

| Field | Meaning |
|---|---|
| Source term | Preferred term in the source language |
| Target term | Preferred term in the target language |
| Code identifier | Type, field, API, or package spelling when relevant |
| Definition | Language-neutral concept boundary |
| Avoid | Ambiguous or deprecated synonyms |

Preserve code identifiers exactly. On first use, introduce the source or canonical term when it helps recognition; afterward, use the natural preferred term consistently.

Do not translate product names, protocol names, type names, or identifiers merely to make the prose look fully localized.

## Semantic parity checks

Compare counterparts by meaning rather than sentence count:

1. Do titles and summaries promise the same answer?
2. Do scope and exclusions match?
3. Are Current, Planned, Exploratory, Deprecated, and Historical labels identical in effect?
4. Are MUST, SHOULD, MAY, guarantees, and possibilities equally strong?
5. Do quantities, timeouts, limits, state names, and ordering rules match?
6. Are failure and recovery behaviors equally complete?
7. Do diagrams use terms recognized by every counterpart?
8. Do links resolve to the appropriate language or shared source?

Translation must not silently turn “may” into “will,” “planned” into “supported,” or “usually” into “always.”

## Suggested metadata

Use repository conventions first. When no convention exists and maintained counterparts need explicit linkage, recommend metadata such as:

```yaml
doc_id: architecture.example
title: "Reader-oriented title"
language: "<BCP-47-language-tag>"
source_language: "<BCP-47-language-tag>"
counterpart: "./<counterpart-filename>.md"
implementation_status: current
document_status: draft
translation_status: synced
last_verified: YYYY-MM-DD
owners:
  - "<team-or-role>"
```

Useful translation states are `synced`, `needs-update`, and `source-only`. Metadata is optional unless the project adopts it; semantic clarity is mandatory.

## Writing voice

In every language version:

- prefer direct, concrete sentences;
- explain necessary jargon at first use;
- write headings as reader-oriented signposts;
- avoid promotional claims and vague adjectives;
- retain the same technical altitude, even when one language needs extra connective explanation;
- use idiomatic target-language syntax and discourse rather than exposing the source language's sentence structure.
