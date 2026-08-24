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

## Local-first y open source

El proyecto está diseñado para ejecutarse localmente y no tendrá un servicio central obligatorio. No habrá backend propio, base de datos hospedada, telemetría, cuentas de usuario ni costos operativos para el mantenedor.

Cada persona conservará sus archivos y resultados en su propio equipo y podrá conectar la biblioteca con su propio Codex, Claude Code u otro agente compatible. Si el agente elegido utiliza un servicio pago, esa relación y ese costo pertenecen al usuario. También debería ser posible trabajar con modelos locales cuando el usuario los tenga disponibles.

El repositorio será open source y debe evitar dependencias que obliguen al proyecto a mantener infraestructura propia. Los apuntes y materiales académicos tendrán una licencia o condición de uso explícita, separada de la licencia del código.

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

La primera versión se concentra en materiales locales y resultados reproducibles. Una biblioteca pública de apuntes, autenticación, colaboración entre estudiantes y búsqueda semántica quedan fuera del núcleo local y solo podrían agregarse como herramientas opcionales que el usuario ejecute o aloje por su cuenta.

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

## Go CLI y MCP local

El MVP incluye un binario Go local. Requiere Go 1.22+ para compilar; SQLite y FTS5 se
incluyen mediante `modernc.org/sqlite`, por lo que no necesita CGO.

```bash
go build -o apuntes ./cmd/apuntes
./apuntes init
./apuntes ingest --path ./materiales
./apuntes search subnetting
./apuntes profile init
./apuntes study-path --subject redes
```

El servidor MCP usa JSON-RPC por `stdio`:

```bash
./apuntes mcp
./apuntes mcp install --agent claude
```

Herramientas disponibles: `listar_materias`, `buscar_material`, `leer_fuente`,
`sugerir_ruta_de_estudio`, `buscar_ejercicios` y `obtener_perfil`. El índice se
guarda en `data/index.db`; el contenido no sale del equipo. Ollama y embeddings
son opcionales y todavía no son necesarios para el fallback FTS.

## Flujo de clase y resúmenes

Registra una clase en vivo (por CLI o MCP) y genera un apunte de repaso:

```bash
./apuntes clase start "Subnetting"
./apuntes clase ask "¿Qué es CIDR?" --respuesta "/24 = 24 bits de red"
./apuntes clase end
./apuntes resumen --pdf   # genera resumenes/<sesion>.md y .pdf
```

Por MCP las tools son `iniciar_clase`, `registrar_pregunta` y `cerrar_clase`:
ideal para preguntarle a tu agente durante la clase y que todo quede registrado.

## Compartir tus apuntes

Este repo es público y los apuntes en `materiales/` se publican a propósito.
Para consultar los apuntes de otro (o compartir los tuyos) no hace falta
ningún servidor:

1. Cloná este repo (o el fork con los apuntes que te interesen).
2. Compilá e indexá:
   ```bash
   go build -o apuntes ./cmd/apuntes
   ./apuntes init && ./apuntes ingest
   ```
3. Consultá con `./apuntes search <query>` o conectá tu agente favorito por MCP
   (`./apuntes mcp install --agent claude|codex`). Cada persona usa su propio
   modelo; el índice y las fuentes quedan locales.
