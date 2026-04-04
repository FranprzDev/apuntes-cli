# Subjects

Cada materia vive en su propia carpeta dentro de `subjects/`.

Estructura sugerida:

```text
subjects/
  nombre-de-materia/
    subject.json
    content.md
    template.html        # opcional, override local
```

Reglas:

- Si una materia no define `template.html`, debe usar `templates/template_base.html`.
- Si una materia necesita una variante visual o estructural, puede definir su propio `template.html`.
- `subject.json` es el punto de configuración para elegir el template de esa materia.
