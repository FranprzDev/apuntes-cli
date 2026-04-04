# apuntes-cli Workflow

Use this skill when the user wants a Codex-driven CLI/plugin that turns local study materials into two outputs:

- a FAQ layer for a subject
- a compacted final report/apunte for that subject

## Goal

- Initialize a reusable workspace with `templates/`, `subjects/`, `faqs/`, and `compactacion/`.
- Create or update one subject at a time.
- Generate a FAQ document from detected local sources.
- Generate the final compacted HTML and PDF for the subject.

## Workflow

1. Run `npm run init` to create the workspace.
2. Run `npm run subject:create -- --subject <materia>` when the subject does not exist yet.
3. Run `npm run sources` to inventory local materials from `input/`.
4. Run `npm run faqs -- --subject <materia>` to build the FAQ draft.
5. Review and refine `subjects/<materia>/content.md` and `faqs/<materia>.md`.
6. Run `npm run compactar -- --subject <materia>` to generate `compactacion/<materia>.html`.
7. Run `npm run export:pdf -- --subject <materia>` to generate `dist/<materia>.pdf`.

## Rules

- Do not invent content that is not supported by the local materials.
- Prefer the shared `templates/template_base.html` unless the subject needs its own `template.html`.
- Keep the HTML self-contained so PDF export works without external assets.
- Treat `faqs/` as the fast consultation layer and `compactacion/` as the final study artifact.

## Output files

- `working/config.json`
- `working/sources.json`
- `subjects/<materia>/subject.json`
- `subjects/<materia>/content.md`
- `faqs/<materia>.md`
- `compactacion/<materia>.html`
- `dist/<materia>.pdf`
