# TL;DR

## Qué vamos a construir

Una herramienta local y open source que distribuya los apuntes de UTN FRT para que cada persona pueda conectarlos a su propio Codex, Claude Code u otro agente compatible.

No será un chatbot centralizado ni tendrá backend, cuentas, telemetría o costos operativos para el mantenedor.

## Arquitectura elegida

```text
apuntes-cli (Go)
├── lectura de apuntes y fuentes
├── índice y búsqueda local
├── exportación opcional a Markdown/HTML/PDF
└── servidor MCP local por stdio
```

El MCP será la interfaz que usarán los agentes. La CLI será el núcleo que organiza, indexa y actualiza los materiales.

## Por qué Go

- Genera un binario único y fácil de distribuir.
- No obliga al usuario a instalar Node.js, npm o Chromium.
- Tiene buen rendimiento y bajo consumo para trabajo local.
- Cuenta con un SDK oficial de MCP.
- Nos permite cambiar componentes internos sin quedar atados a una sola tecnología.

Node/TypeScript sería más rápido para prototipar, pero requiere más dependencias. Rust ofrece gran rendimiento y seguridad, aunque agrega complejidad y su SDK de MCP todavía tiene menor madurez para este caso.

## Principios

- Las respuestas deben basarse en las fuentes cargadas.
- El agente debe indicar cuándo no existe evidencia suficiente.
- Los materiales deben conservar referencias y separación por materia.
- Los apuntes y el código deben tener condiciones de uso claras y separadas.
- El usuario debe poder usar sus propios modelos y su propio agente.

## Primera versión

1. Reescribir el núcleo actual en Go.
2. Implementar lectura e índice local de fuentes.
3. Exponer búsqueda y consulta mediante MCP.
4. Integrar el servidor con Claude Code y Codex.
5. Agregar exportación de materiales como función opcional.
