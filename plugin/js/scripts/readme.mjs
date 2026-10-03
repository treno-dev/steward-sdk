// Replaces each marked block in README.md with the file at the path in its marker, so an example is
// written once and the README never drifts from what `steward plugin init` generates. Paths are
// relative to the SDK package, and placeholders in a template get sample values.
//
// A block looks like this, and everything between the code fences is replaced:
//
//   <!-- template: ../../templates/js/src/plugin.js.tmpl -->
//   ```js
//   ```
//   <!-- /template -->
//
//   node scripts/readme.mjs           rewrite README.md
//   node scripts/readme.mjs --check   exit 1 if README.md is out of date

import { readFileSync, writeFileSync } from 'node:fs';

const readme = new URL('../README.md', import.meta.url);
const root = new URL('../', import.meta.url);

// The values the CLI fills in when it renders a template.
const values = { Name: 'my-plugin', Title: 'My plugin', SDK: '^0.1.0' };

const render = (text) => text.replace(/\[\[\.(\w+)\]\]/g, (_, key) => values[key] ?? `[[.${key}]]`);

const block = /(<!-- template: (\S+) -->\n```\w*\n)[\s\S]*?(```\n<!-- \/template -->)/g;

const current = readFileSync(readme, 'utf8');

const next = current.replace(block, (_, open, file, close) => {
  const source = render(readFileSync(new URL(file, root), 'utf8'));

  return `${open}${source.endsWith('\n') ? source : `${source}\n`}${close}`;
});

if (process.argv.includes('--check')) {
  if (next !== current) {
    console.error('README.md is out of date: run `npm run readme`.');
    process.exit(1);
  }

  process.exit(0);
}

writeFileSync(readme, next);
