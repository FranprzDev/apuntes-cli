# apuntes-cli

[![CI](https://github.com/FranprzDev/apuntes-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/FranprzDev/apuntes-cli/actions/workflows/ci.yml)

`apuntes-cli` es una CLI local (y servidor MCP) que convierte material de estudio en recursos trazables: índice de búsqueda, rutas de estudio, registro de clases, resúmenes y progreso.

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

## Principios del producto

- **Fuentes primero:** las respuestas y materiales deben basarse en archivos efectivamente cargados.
- **Trazabilidad:** cada respuesta debería poder indicar de qué fuente y sección proviene.
- **Sin invenciones:** si el material no alcanza para responder, la herramienta debe decirlo y pedir otra fuente o reformular la pregunta.
- **Materias separadas:** cada materia mantiene su configuración, fuentes, FAQs y resultados.
- **Uso local y compartible:** el estudiante puede trabajar con sus archivos localmente y compartir los resultados que decida publicar.

## Requisitos

- Go 1.22+ para compilar. SQLite y FTS5 se incluyen mediante `modernc.org/sqlite`, así que no necesita CGO.
- Un agente compatible con MCP (opcional): Claude Code, Codex u otro.

## Instalación

Compilá el binario:

```bash
go build -o apuntes ./cmd/apuntes
```

O instalalo directo:

```bash
go install github.com/franciscoperez/apuntes-cli/cmd/apuntes@latest
```

Las releases también publican binarios para Linux/macOS/Windows vía [goreleaser](https://goreleaser.com).

## Uso básico

```bash
./apuntes init                    # crea data/ y materiales/
./apuntes ingest                  # indexa materiales/ (md, txt y pdf)
./apuntes search subnetting       # búsqueda full-text (salida JSON)
./apuntes doctor                  # verifica la salud del workspace
```

Con perfil y ruta de estudio:

```bash
./apuntes profile init            # institución, carrera, materias activas
./apuntes study-path --subject redes
```

Los filtros por materias activas del perfil se aplican automáticamente a `search` y `study-path`.

## Flujo de clase y resúmenes

Registra una clase en vivo (por CLI o MCP) y genera un apunte de repaso:

```bash
./apuntes clase start "Subnetting"
./apuntes clase ask "¿Qué es CIDR?" --respuesta "/24 = 24 bits de red"
./apuntes clase end
./apuntes resumen --pdf   # genera resumenes/<sesion>.md y .pdf
```

## Servidor MCP local

El servidor MCP usa JSON-RPC por `stdio`:

```bash
./apuntes mcp
./apuntes mcp install --agent claude   # imprime el JSON de configuración
```

Herramientas disponibles: `listar_materias`, `buscar_material`, `leer_fuente`, `sugerir_ruta_de_estudio`, `buscar_ejercicios`, `obtener_perfil`, `iniciar_clase`, `registrar_pregunta`, `cerrar_clase`, `guardar_progreso`, `resumir_historial` y `generar_resumen`. El índice se guarda en `data/index.db`; el contenido nunca sale del equipo.

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

## Estructura

```text
cmd/apuntes/        binario principal
internal/core/      App, índice SQLite/FTS5, ingest, búsqueda, perfil
internal/session/   sesiones de clase
internal/progress/  progreso por tema
internal/summary/   generación de resúmenes md/pdf
internal/mcp/       servidor MCP (JSON-RPC stdio)
internal/cli/       comandos y ayuda
materiales/         fuentes de estudio (publicadas a propósito)
```

## Notas

- El antiguo plugin Node (`plugins/apuntes-cli`) fue removido; toda la funcionalidad vive hoy en el binario Go.
- El contenido de UTN FRT debe publicarse respetando la autoría, las condiciones de uso de los materiales y la privacidad de quienes compartan apuntes.
