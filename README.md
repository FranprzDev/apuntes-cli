# apuntes-cli

This repository contains `apuntes-cli`, a portable CLI/plugin for turning local study materials into useful, traceable study resources.

## Product vision

`apuntes-cli` nace para compartir los apuntes de la Facultad Regional Tucumán de la UTN (UTN FRT) y permitir que cualquier estudiante pueda:

- consultar apuntes organizados por materia;
- estudiar con FAQs, resúmenes y apuntes compactados;
- hacer preguntas sobre el material disponible;
- recibir respuestas fundamentadas en las fuentes cargadas, con referencias claras;
- reutilizar el flujo con sus propios materiales y materias.

La herramienta debe funcionar como una biblioteca de estudio asistida por agentes. Codex, Claude Code u otro agente compatible pueden ayudar a procesar y consultar el contenido, pero el CLI debe seguir siendo portable y no depender de una cuenta, modelo o proveedor específico.

## Principios del producto

- **Fuentes primero:** las respuestas y materiales deben basarse en archivos efectivamente cargados.
- **Trazabilidad:** cada respuesta debería poder indicar de qué fuente y sección proviene.
- **Sin invenciones:** si el material no alcanza para responder, la herramienta debe decirlo y pedir otra fuente o reformular la pregunta.
- **Materias separadas:** cada materia mantiene su configuración, fuentes, FAQs y resultados.
- **Uso local y compartible:** el estudiante puede trabajar con sus archivos localmente y compartir los resultados que decida publicar.
- **Agente desacoplado:** Gentleman AI se utiliza para desarrollar y revisar el proyecto; los usuarios finales pueden utilizar Codex, Claude Code u otro agente compatible.

## Experiencia esperada

Un estudiante debería poder:

1. Instalar `apuntes-cli` o el plugin del agente que prefiera.
2. Crear o seleccionar una materia de UTN FRT.
3. Cargar PDFs, documentos, clases, ejercicios y apuntes propios.
4. Generar FAQs y un apunte compacto en HTML/PDF.
5. Preguntar algo como “¿cómo se aplica este concepto?” y obtener una respuesta basada en sus fuentes.
6. Revisar las referencias antes de usar la respuesta para estudiar.

La primera versión se concentra en materiales locales y resultados reproducibles. Una biblioteca pública de apuntes, autenticación, colaboración entre estudiantes y búsqueda semántica quedan como extensiones posteriores.

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
- El contenido de UTN FRT debe publicarse respetando la autoría, las condiciones de uso de los materiales y la privacidad de quienes compartan apuntes.
