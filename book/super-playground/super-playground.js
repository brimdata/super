import {AriaTabs} from './aria-tabs';
import {Editor} from './editor';
import {superdb} from './superdb';

class SuperPlayground {
  static setup(node) {
    const playground = new SuperPlayground(node);
    node.__super_playground__ = playground;
    playground.setup();
  }

  static teardown(node) {
    const playground = node.__super_playground__;
    if (playground) {
      playground.teardown();
      delete node.__super_playground__;
    }
  }

  constructor(node) {
    this.node = node;
  }

  setup() {
    this.input = new Editor({
      node: this.node.querySelector('.input pre'),
      onChange: () => this.run()
    });
    this.query = new Editor({
      node: this.node.querySelector('.query pre'),
      onChange: () => this.run(),
      language: 'sql'
    });
    this.result = new Editor({
      node: this.node.querySelector('.result pre')
    });
    this.run();
  }

  teardown() {
    this.input.teardown();
    this.query.teardown();
    this.result.teardown();
  }

  async run() {
    this.result.value = await superdb({
      query: this.query.value,
      input: this.input.value
    });
  }
}

const preNodes = document.querySelectorAll('pre:has(> code.language-mdtest-spq)');
for (const [i, pre] of preNodes.entries()) {
  const codeNode = pre.querySelector('code')

  const codeText = codeNode.innerText;
  const matches = Array.from(codeText.matchAll(/(?:#[^\n]*\n)+((?:[^#][^\n]*\n)+)/gm));
  if (matches.length != 3) {
    continue;
  }
  const [spq, input, expected] = [matches[0][1], matches[1][1], matches[2][1]];

  const attributes = Array.from(codeNode.classList)
        .filter((c) => c.match(/^{.*}$/))
        .map((c) => c.slice(1, -1))
        .join(' ')

  const html = `
  <article class="super-command-example">
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
        aria-controls="shell-panel-${i}"
        id="shell-tab-${i}"
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
    <section hidden role="tabpanel" id="shell-panel-${i}" class="shell-command">
      <pre><code>echo '${input}' \
| super -s -c '${spq}' -</code></pre>
    </section>
  </article>
`;

  const div = document.createElement('div');
  div.innerHTML = html;
  const node = div.children[0]
  pre.replaceWith(node);

  for (const tablist of node.querySelectorAll('[role="tablist"]')) {
    AriaTabs.setup(tablist);
  }

  SuperPlayground.setup(node);

  // Prevent keydown from bubbling up to book.js listeners.
  node.addEventListener('keydown', (e) => e.stopPropagation());
}
