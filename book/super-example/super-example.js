import { SuperPlayground } from './super-playground';

const preNodes = document.querySelectorAll('pre:has(> code.language-mdtest-spq)');
for (const [i, pre] of preNodes.entries()) {
  // mdBook creates a <code> element for each fenced code block.
  const codeNode = pre.querySelector('code');

  // Matches one or more "#"-prefixed lines.
  const sectionSeparatorRE = /(?m:^#.*\n)+/;
  const sections = codeNode.innerText.split(sectionSeparatorRE);
  // Ignore sections[0], which should be empty.
  if (sections.length != 4) {
    continue;
  }
  const spq = sections[1].trim();
  const input = sections[2].trim();
  const expected = sections[3].trim();

  // mdBook creates a <code> element's class list by splitting the
  // corresponding fenced block's info string on ' ', '\t', and ','.
  //
  // Replace '&' with ' ' so attributes can contain spaces.
  const attributes = Array.from(codeNode.classList)
        .filter((c) => c.match(/^{.*}$/))
        .map((c) => c.slice(1, -1).replaceAll('&', ' '))
        .join(' ');

  const html = `
  <article class="super-example ${input.length === 0 ? 'no-input' : ''}" ${attributes}>
    <div class="editor query">
      <header class="repel"><label>Query</label></header>
      <pre><code></code></pre>
    </div>
    <div class="editor input">
      <header class="repel"><label>Input</label></header>
      <pre><code></code></pre>
    </div>
    <div class="editor result">
      <header class="repel"><label>Result</label></header>
      <pre><code></code></pre>
    </div>
  </article>
`;

  const div = document.createElement('div');
  div.innerHTML = html;
  const node = div.children[0];
  pre.replaceWith(node);

  node.querySelector('.query code').textContent = spq;
  node.querySelector('.input code').textContent = input;
  node.querySelector('.result code').textContent = expected;

  SuperPlayground.setup(node, null);

  // Prevent keydown from bubbling up to book.js listeners.
  node.addEventListener('keydown', (e) => e.stopPropagation());
}
