# apuntes-cli

This repository now contains `apuntes-cli`, a Codex-oriented CLI/plugin for generating study materials in two stages: `faqs` and `compactacion`.

## What it does

- Scans local source files and builds a source inventory.
- Creates a reusable workspace with shared templates and per-subject configuration.
- Generates a FAQ file for each subject.
- Compacts a subject into a final HTML apunte and exports it to PDF.

## Repository layout

```text
plugins/apuntes-cli/
  .codex-plugin/plugin.json
  package.json
  scripts/
  skills/
  templates/
```

## Local workflow

1. Put source files in `plugins/apuntes-cli/input/`.
2. Run `npm install` inside `plugins/apuntes-cli/`.
3. Install the browser once with `npx playwright install chromium`.
4. Run `npm run init`.
5. Run `npm run subject:create -- --subject "nombre de materia"`.
6. Run `npm run sources`.
7. Run `npm run faqs -- --subject "nombre de materia"`.
8. Run `npm run compactar -- --subject "nombre de materia"`.
9. Run `npm run export:pdf -- --subject "nombre de materia"`.

You can also run the full pipeline with:

```bash
npm run full -- --subject "nombre de materia"
```

## Templates and subjects

- The shared reusable base template lives in `templates/template_base.html`.
- Subject-specific content and configuration live under `subjects/`.
- Each subject can point to the base template or override it with its own local `template.html`.
- FAQ output is written to `faqs/`.
- Final compacted HTML output is written to `compactacion/`.

## Notes

- The old duplicated root-level templates were replaced by the shared `templates/` + `subjects/` structure.
- Fill in the manifest placeholders before publishing the repo publicly.
