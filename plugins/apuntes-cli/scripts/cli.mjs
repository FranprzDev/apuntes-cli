#!/usr/bin/env node

import { promises as fs } from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath, pathToFileURL } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const pluginRoot = path.resolve(__dirname, '..');
const assetTemplatePath = path.join(pluginRoot, 'templates', 'template_base.html');

function parseArgs(argv) {
  const [command = 'help', ...rest] = argv;
  const options = {};

  for (let index = 0; index < rest.length; index += 1) {
    const token = rest[index];

    if (!token.startsWith('--')) {
      continue;
    }

    const key = token.slice(2);
    const next = rest[index + 1];
    if (!next || next.startsWith('--')) {
      options[key] = true;
      continue;
    }

    options[key] = next;
    index += 1;
  }

  return { command, options };
}

function slugify(value) {
  return value
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .replace(/-{2,}/g, '-');
}

async function pathExists(targetPath) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

async function ensureDir(targetPath) {
  await fs.mkdir(targetPath, { recursive: true });
}

async function copyIfMissing(sourcePath, destinationPath) {
  if (await pathExists(destinationPath)) {
    return false;
  }

  await ensureDir(path.dirname(destinationPath));
  await fs.copyFile(sourcePath, destinationPath);
  return true;
}

async function writeJsonIfMissing(targetPath, payload) {
  if (await pathExists(targetPath)) {
    return false;
  }

  await ensureDir(path.dirname(targetPath));
  await fs.writeFile(targetPath, `${JSON.stringify(payload, null, 2)}\n`, 'utf8');
  return true;
}

async function writeTextIfMissing(targetPath, content) {
  if (await pathExists(targetPath)) {
    return false;
  }

  await ensureDir(path.dirname(targetPath));
  await fs.writeFile(targetPath, content, 'utf8');
  return true;
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function inlineMarkdown(value) {
  return escapeHtml(value)
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/`(.+?)`/g, '<code>$1</code>');
}

function markdownToHtml(markdown) {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const chunks = [];
  let paragraph = [];
  let listItems = [];
  let inCodeBlock = false;
  let codeLines = [];

  const flushParagraph = () => {
    if (paragraph.length === 0) {
      return;
    }
    chunks.push(`<p>${inlineMarkdown(paragraph.join(' '))}</p>`);
    paragraph = [];
  };

  const flushList = () => {
    if (listItems.length === 0) {
      return;
    }
    chunks.push(`<ul>${listItems.map((item) => `<li>${inlineMarkdown(item)}</li>`).join('')}</ul>`);
    listItems = [];
  };

  const flushCode = () => {
    if (codeLines.length === 0) {
      return;
    }
    chunks.push(`<div class="formula">${escapeHtml(codeLines.join('\n'))}</div>`);
    codeLines = [];
  };

  for (const rawLine of lines) {
    const line = rawLine.trimEnd();
    const trimmed = line.trim();

    if (trimmed.startsWith('```')) {
      flushParagraph();
      flushList();
      if (inCodeBlock) {
        flushCode();
      }
      inCodeBlock = !inCodeBlock;
      continue;
    }

    if (inCodeBlock) {
      codeLines.push(line);
      continue;
    }

    if (trimmed === '') {
      flushParagraph();
      flushList();
      continue;
    }

    if (trimmed.startsWith('### ')) {
      flushParagraph();
      flushList();
      chunks.push(`<h3>${inlineMarkdown(trimmed.slice(4))}</h3>`);
      continue;
    }

    if (trimmed.startsWith('## ')) {
      flushParagraph();
      flushList();
      chunks.push(`<h2>${inlineMarkdown(trimmed.slice(3))}</h2>`);
      continue;
    }

    if (trimmed.startsWith('# ')) {
      flushParagraph();
      flushList();
      chunks.push(`<h2>${inlineMarkdown(trimmed.slice(2))}</h2>`);
      continue;
    }

    if (/^- /.test(trimmed)) {
      flushParagraph();
      listItems.push(trimmed.slice(2));
      continue;
    }

    paragraph.push(trimmed);
  }

  flushParagraph();
  flushList();
  flushCode();

  return chunks.join('\n');
}

function applyTemplate(template, values) {
  return template.replace(/\{\{([a-zA-Z0-9]+)(?:\|([^}]+))?\}\}/g, (_, key, fallback = '') => {
    if (values[key] !== undefined && values[key] !== null && values[key] !== '') {
      return String(values[key]);
    }
    return fallback;
  });
}

function inventorySummary(files) {
  if (files.length === 0) {
    return 'No se encontraron fuentes todavia.';
  }

  const preview = files
    .slice(0, 6)
    .map((file) => `- \`${file.relativePath}\` (${file.kind}, ${file.extension || 'sin extension'})`)
    .join('\n');

  return `Se encontraron ${files.length} archivo(s):\n${preview}`;
}

async function collectSources(workspaceRoot, options = {}) {
  const inputDir = path.resolve(workspaceRoot, options.input ?? 'input');
  const outputFile = path.resolve(workspaceRoot, options.output ?? path.join('working', 'sources.json'));
  const textExtensions = new Set([
    '.txt',
    '.md',
    '.markdown',
    '.html',
    '.htm',
    '.json',
    '.csv',
    '.yml',
    '.yaml',
    '.xml',
    '.js',
    '.mjs',
    '.ts',
    '.py',
    '.pdf'
  ]);

  async function walk(dirPath, relativeBase = '') {
    const entries = [];
    const dirEntries = await fs.readdir(dirPath, { withFileTypes: true });

    for (const entry of dirEntries) {
      if (entry.name.startsWith('.')) {
        continue;
      }

      const absolutePath = path.join(dirPath, entry.name);
      const relativePath = path.join(relativeBase, entry.name);

      if (entry.isDirectory()) {
        entries.push(...await walk(absolutePath, relativeBase ? relativePath : entry.name));
        continue;
      }

      const stats = await fs.stat(absolutePath);
      const extension = path.extname(entry.name).toLowerCase();
      entries.push({
        relativePath: relativePath.split(path.sep).join('/'),
        extension,
        kind: textExtensions.has(extension) ? 'text' : 'binary',
        sizeBytes: stats.size,
        modifiedTime: stats.mtime.toISOString()
      });
    }

    return entries;
  }

  const files = (await pathExists(inputDir)) ? await walk(inputDir) : [];
  const payload = {
    generatedAt: new Date().toISOString(),
    inputDir: path.relative(workspaceRoot, inputDir) || '.',
    fileCount: files.length,
    textFiles: files.filter((file) => file.kind === 'text').length,
    binaryFiles: files.filter((file) => file.kind === 'binary').length,
    files
  };

  await ensureDir(path.dirname(outputFile));
  await fs.writeFile(outputFile, `${JSON.stringify(payload, null, 2)}\n`, 'utf8');
  return { outputFile, payload };
}

async function ensureWorkspace(workspaceRoot) {
  const folders = ['input', 'templates', 'subjects', 'faqs', 'compactacion', 'dist', 'working'];
  for (const folder of folders) {
    await ensureDir(path.join(workspaceRoot, folder));
  }

  const createdTemplate = await copyIfMissing(
    assetTemplatePath,
    path.join(workspaceRoot, 'templates', 'template_base.html')
  );

  const createdConfig = await writeJsonIfMissing(path.join(workspaceRoot, 'working', 'config.json'), {
    projectName: 'apuntes-cli',
    defaultTemplate: './templates/template_base.html',
    inputDir: './input',
    subjectsDir: './subjects',
    faqsDir: './faqs',
    compactacionDir: './compactacion',
    distDir: './dist'
  });

  return { createdTemplate, createdConfig };
}

async function createSubject(workspaceRoot, options) {
  const rawName = options.subject ?? options.name;
  if (!rawName) {
    throw new Error('Missing --subject for subject:create');
  }

  const slug = slugify(rawName);
  const subjectDir = path.join(workspaceRoot, 'subjects', slug);
  const title = options.title ?? rawName;
  const useLocalTemplate = Boolean(options['local-template']);
  const subjectConfig = {
    name: slug,
    title,
    eyebrow: options.eyebrow ?? 'Apunte de materia',
    subtitle: options.subtitle ?? `Resumen y compactacion de ${title}.`,
    template: useLocalTemplate ? './template.html' : '../../templates/template_base.html',
    content: './content.md',
    faq: `../../faqs/${slug}.md`,
    outputHtml: `../../compactacion/${slug}.html`,
    outputPdf: `../../dist/${slug}.pdf`
  };

  const createdConfig = await writeJsonIfMissing(path.join(subjectDir, 'subject.json'), subjectConfig);
  const createdContent = await writeTextIfMissing(
    path.join(subjectDir, 'content.md'),
    `# ${title}\n\n## Idea general\n\nEscribi aca el contenido base de la materia.\n\n## Puntos clave\n\n- Punto 1\n- Punto 2\n- Punto 3\n`
  );

  let createdTemplate = false;
  if (useLocalTemplate) {
    createdTemplate = await copyIfMissing(
      assetTemplatePath,
      path.join(subjectDir, 'template.html')
    );
  }

  return {
    slug,
    createdConfig,
    createdContent,
    createdTemplate,
    subjectDir
  };
}

async function readSubjectConfig(workspaceRoot, subjectName) {
  const slug = slugify(subjectName);
  const subjectDir = path.join(workspaceRoot, 'subjects', slug);
  const configPath = path.join(subjectDir, 'subject.json');

  if (!(await pathExists(configPath))) {
    throw new Error(`Subject not found: ${slug}. Run subject:create first.`);
  }

  const config = JSON.parse(await fs.readFile(configPath, 'utf8'));
  return {
    slug,
    subjectDir,
    config,
    configPath
  };
}

async function generateFaq(workspaceRoot, options) {
  const subjectName = options.subject;
  if (!subjectName) {
    throw new Error('Missing --subject for faq');
  }

  const { slug, config, subjectDir } = await readSubjectConfig(workspaceRoot, subjectName);
  const { payload } = await collectSources(workspaceRoot);
  const faqPath = path.resolve(subjectDir, config.faq);
  const title = config.title ?? slug;
  const content = [
    `# FAQ - ${title}`,
    '',
    '## Como estudiar esta materia',
    '- Que temas son obligatorios para entender el apunte.',
    '- Que parte suele generar mas dudas en defensa o examen.',
    '- Que formulas, definiciones o pasos conviene memorizar.',
    '',
    '## Dudas frecuentes',
    '- Que pregunta te harian primero sobre este tema.',
    '- Cual es la diferencia entre los conceptos que mas se confunden.',
    '- Que error tipico conviene evitar al explicar el tema.',
    '',
    '## Fuentes detectadas',
    inventorySummary(payload.files),
    '',
    '## Proximos pasos',
    '- Reemplazar estas preguntas por respuestas concretas.',
    '- Vincular cada respuesta a apuntes, PDFs o notas de la carpeta `input/`.'
  ].join('\n');

  await ensureDir(path.dirname(faqPath));
  await fs.writeFile(faqPath, `${content}\n`, 'utf8');
  return { faqPath, slug };
}

function renderSourcesSection(payload) {
  const items = payload.files.map((file) => (
    `<li><strong>${escapeHtml(file.relativePath)}</strong> <em>${escapeHtml(file.kind)}</em> ${escapeHtml(file.extension || 'sin extension')}</li>`
  )).join('');

  return [
    '<section class="section">',
    '<h2>Fuentes detectadas</h2>',
    payload.files.length === 0
      ? '<p>No se encontraron fuentes en la carpeta <code>input/</code>.</p>'
      : `<ul>${items}</ul>`,
    '</section>'
  ].join('');
}

async function compactar(workspaceRoot, options) {
  const subjectName = options.subject;
  if (!subjectName) {
    throw new Error('Missing --subject for compactar');
  }

  const { slug, config, subjectDir } = await readSubjectConfig(workspaceRoot, subjectName);
  const contentPath = path.resolve(subjectDir, config.content);
  const faqPath = path.resolve(subjectDir, config.faq);
  const templatePath = path.resolve(subjectDir, config.template);
  const outputHtml = path.resolve(subjectDir, config.outputHtml);
  const sourceInventoryPath = path.join(workspaceRoot, 'working', 'sources.json');

  if (!(await pathExists(contentPath))) {
    throw new Error(`Missing content file: ${contentPath}`);
  }

  if (!(await pathExists(faqPath))) {
    await generateFaq(workspaceRoot, { subject: subjectName });
  }

  const contentMarkdown = await fs.readFile(contentPath, 'utf8');
  const faqMarkdown = await fs.readFile(faqPath, 'utf8');
  const template = await fs.readFile(templatePath, 'utf8');
  const payload = (await pathExists(sourceInventoryPath))
    ? JSON.parse(await fs.readFile(sourceInventoryPath, 'utf8'))
    : { files: [] };

  const contentHtml = [
    '<section class="section">',
    markdownToHtml(contentMarkdown),
    '</section>',
    '<section class="section">',
    '<h2>FAQ</h2>',
    markdownToHtml(faqMarkdown),
    '</section>',
    renderSourcesSection(payload)
  ].join('\n');

  const html = applyTemplate(template, {
    eyebrow: config.eyebrow ?? 'Apunte de materia',
    title: config.title ?? slug,
    subtitle: config.subtitle ?? `Resumen y compactacion de ${config.title ?? slug}.`,
    content: contentHtml,
    footerLeft: 'Generado por apuntes-cli',
    footerRight: new Date().toISOString().slice(0, 10)
  });

  await ensureDir(path.dirname(outputHtml));
  await fs.writeFile(outputHtml, html, 'utf8');
  return { outputHtml, slug };
}

async function exportPdf(workspaceRoot, options) {
  const subjectName = options.subject;
  if (!subjectName) {
    throw new Error('Missing --subject for export-pdf');
  }

  const { config, subjectDir } = await readSubjectConfig(workspaceRoot, subjectName);
  const inputFile = path.resolve(subjectDir, config.outputHtml);
  const outputFile = path.resolve(subjectDir, config.outputPdf);

  if (!(await pathExists(inputFile))) {
    await compactar(workspaceRoot, { subject: subjectName });
  }

  let chromium;
  try {
    ({ chromium } = await import('playwright'));
  } catch {
    throw new Error('Playwright is not installed. Run npm install and npx playwright install chromium.');
  }

  await ensureDir(path.dirname(outputFile));

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({
      viewport: { width: 1240, height: 1754 }
    });

    await page.goto(pathToFileURL(inputFile).href, { waitUntil: 'networkidle' });
    await page.emulateMedia({ media: 'print' });
    await page.pdf({
      path: outputFile,
      format: 'A4',
      printBackground: true,
      preferCSSPageSize: true,
      margin: {
        top: '18mm',
        right: '16mm',
        bottom: '18mm',
        left: '16mm'
      }
    });
  } finally {
    await browser.close();
  }

  return { outputFile };
}

function printHelp() {
  console.log(`apuntes-cli

Commands:
  init [--subject nombre] [--title titulo] [--local-template]
  subject:create --subject nombre [--title titulo] [--local-template]
  sources
  faq --subject nombre
  compactar --subject nombre
  export-pdf --subject nombre
  full --subject nombre
`);
}

async function main() {
  const workspaceRoot = process.cwd();
  const { command, options } = parseArgs(process.argv.slice(2));

  if (command === 'help' || command === '--help' || command === '-h') {
    printHelp();
    return;
  }

  if (command === 'init') {
    const result = await ensureWorkspace(workspaceRoot);
    console.log(`Workspace ready at ${workspaceRoot}`);
    console.log(`Base template: ${result.createdTemplate ? 'created' : 'already existed'}`);
    console.log(`Config: ${result.createdConfig ? 'created' : 'already existed'}`);
    if (options.subject || options.name) {
      const subject = await createSubject(workspaceRoot, options);
      console.log(`Subject ${subject.slug}: ready`);
    }
    return;
  }

  if (command === 'subject:create') {
    await ensureWorkspace(workspaceRoot);
    const subject = await createSubject(workspaceRoot, options);
    console.log(`Created subject scaffold at ${subject.subjectDir}`);
    return;
  }

  if (command === 'sources') {
    await ensureWorkspace(workspaceRoot);
    const { outputFile, payload } = await collectSources(workspaceRoot, options);
    console.log(`Wrote source inventory to ${path.relative(workspaceRoot, outputFile)}`);
    console.log(`Found ${payload.fileCount} source file(s)`);
    return;
  }

  if (command === 'faq') {
    await ensureWorkspace(workspaceRoot);
    const result = await generateFaq(workspaceRoot, options);
    console.log(`Generated FAQ for ${result.slug} at ${path.relative(workspaceRoot, result.faqPath)}`);
    return;
  }

  if (command === 'compactar') {
    await ensureWorkspace(workspaceRoot);
    const result = await compactar(workspaceRoot, options);
    console.log(`Generated HTML at ${path.relative(workspaceRoot, result.outputHtml)}`);
    return;
  }

  if (command === 'export-pdf') {
    await ensureWorkspace(workspaceRoot);
    const result = await exportPdf(workspaceRoot, options);
    console.log(`Generated PDF at ${path.relative(workspaceRoot, result.outputFile)}`);
    return;
  }

  if (command === 'full') {
    await ensureWorkspace(workspaceRoot);
    await collectSources(workspaceRoot, options);
    const faqResult = await generateFaq(workspaceRoot, options);
    const compactResult = await compactar(workspaceRoot, options);
    const pdfResult = await exportPdf(workspaceRoot, options);
    console.log(`FAQ: ${path.relative(workspaceRoot, faqResult.faqPath)}`);
    console.log(`HTML: ${path.relative(workspaceRoot, compactResult.outputHtml)}`);
    console.log(`PDF: ${path.relative(workspaceRoot, pdfResult.outputFile)}`);
    return;
  }

  throw new Error(`Unknown command: ${command}`);
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
