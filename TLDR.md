# TL;DR

## Qué vamos a construir

Un MCP local y open source que distribuya los apuntes de UTN FRT para que cada persona pueda conectarlos a su propio Codex, Claude Code u otro agente compatible.

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

## Experiencia del estudiante

Cada usuario tendrá un perfil local con información general:

```json
{
  "institucion": "UTN FRT",
  "carrera": "Ingeniería en Sistemas",
  "año": 2,
  "materias_activas": ["Redes", "Programación III"],
  "objetivo": "preparar parciales"
}
```

El perfil permitirá priorizar los materiales adecuados para su carrera, año, materias y objetivo. Las materias anteriores no se eliminan: quedan disponibles como base previa cuando una explicación las necesita.

Ejemplo:

```text
Estudiante: No entiendo cómo se subnetea.

MCP:
1. Detecta que Redes es una materia activa.
2. Busca en PDFs, apuntes y ejercicios relacionados.
3. Sugiere una ruta: máscaras, CIDR, subredes y práctica.
4. Devuelve fuentes y rutas locales para que la IA explique el tema.
5. Guarda la dificultad o el avance si el estudiante lo permite.
```

## Capacidades del MCP

Primera versión:

- `listar_materias`: muestra materias disponibles y activas.
- `buscar_material`: busca conceptos en apuntes, PDFs y ejercicios.
- `leer_fuente`: recupera una fuente o fragmento con referencia.
- `sugerir_ruta_de_estudio`: ordena prerequisitos y materiales.
- `buscar_ejercicios`: encuentra práctica relacionada con un tema.

Segunda versión:

- `guardar_progreso`: registra temas estudiados, dificultades y próximos pasos.
- `actualizar_perfil`: modifica carrera, año, materias y objetivos.
- `resumir_historial`: devuelve el estado de aprendizaje local.

El MCP entrega contexto y fuentes; Claude, Codex u otro agente redacta la explicación. No se entrena un modelo ni se envían datos a un servidor propio.

## Organización local de datos

```text
data/
├── materias/
│   └── redes/
│       ├── materia.json
│       ├── fuentes/
│       ├── ejercicios/
│       └── indice.json
├── perfil.json
└── progreso.json
```

El índice debe conservar el archivo, la ruta, la materia, el año, los temas y la ubicación relevante del contenido. El perfil filtra y prioriza resultados sin borrar fuentes.

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

1. Definir el formato de materias, fuentes y perfil.
2. Reescribir el núcleo actual en Go.
3. Implementar lectura e índice local de Markdown, texto y PDF.
4. Exponer búsqueda, lectura y materias mediante MCP por stdio.
5. Implementar filtros por carrera, año, materia y objetivo.
6. Integrar el servidor con Claude Code y Codex.
7. Agregar rutas de estudio y búsqueda de ejercicios.
8. Incorporar progreso local como función opcional.
9. Agregar exportación de materiales como función separada.

## Fuera del núcleo

- chatbot o aplicación web centralizada;
- cuentas y autenticación propias;
- almacenamiento remoto obligatorio;
- entrenamiento de modelos;
- telemetría;
- eliminación automática de materias por año;
- publicación de apuntes sin revisar autoría y condiciones de uso.
