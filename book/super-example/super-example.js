import {AriaTabs} from './aria-tabs';
import {SuperPlayground} from './super-playground'

const preNodes = document.querySelectorAll('pre:has(> code.language-mdtest-spq)');
for (const [i, pre] of preNodes.entries()) {
  const codeNode = pre.querySelector('code')

  const codeText = codeNode.innerText;
  const matches = Array.from(codeText.matchAll(/(?:#[^\n]*\n)+((?:[^#][^\n]*\n)+)/gm));
  if (matches.length != 3) {
    continue;
  }
  const spq = matches[0][1].trim();
  const input = matches[1][1].trim();
  const expected = matches[2][1].trim();

  const attributes = Array.from(codeNode.classList)
        .filter((c) => c.match(/^{.*}$/))
        .map((c) => c.slice(1, -1))
        .join(' ')

  const html = `
  <article class="super-example">
    <nav role="tablist">
      <button
        role="tab"
        aria-selected="true"
        aria-controls="playground-panel-${i}"
        id="playground-tab-${i}"
        tabindex="0"
      >
        Interactive
      </button>
      <button
        role="tab"
        aria-selected="false"
        aria-controls="command-panel-${i}"
        id="command-tab-${i}"
        tabindex="-1"
      >
        CLI
      </button>
    </nav>
    <section
      role="tabpanel"
      id="playground-panel-${i}"
      class="super-playground"
      ${attributes}
    >
      <div class="editor query">
        <header class="repel">
          <label>Query</label>
        </header>
        <pre><code>${spq}</code></pre>
      </div>
      <div class="editor input">
        <header class="repel">
          <label>Input</label>
        </header>
        <pre><code>${input}</code></pre>
      </div>
      <div class="editor result">
        <header class="repel">
          <label>Result</label>
        </header>
        <pre><code>${expected}</code></pre>
      </div>
    </section>
    <section hidden role="tabpanel" id="command-panel-${i}" class="super-command">
      <pre><code></code></pre>
    </section>
  </article>
`;

  const div = document.createElement('div');
  div.innerHTML = html;
  const node = div.children[0]
  pre.replaceWith(node);

  const tablist = node.querySelector('[role="tablist"]');
  AriaTabs.setup(tablist);

  const commandCode = node.querySelector('.super-command code')
  SuperPlayground.setup(node, (query, input) => {
    commandCode.textContent = `echo '${input}' \\\n| super -s -c '${query}' -`
  });

  // Prevent keydown from bubbling up to book.js listeners.
  node.addEventListener('keydown', (e) => e.stopPropagation());
}
