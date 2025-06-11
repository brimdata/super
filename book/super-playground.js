import {Editor} from './editor.js'

function wrapper(goFunc) {
    return (...args) => {
        const result = goFunc.apply(undefined, args);
        if (result.error instanceof Error) {
            throw result.error;
        }
        return result.result;
    };
}

globalThis.__go_wasm__ = {
    __wrapper__: wrapper,
    __ready__: false,
};

class SuperDB {
    /**
     * Instantiates a new `SuperDB` instance from a given wasm file URL.
     * @static
     * @param {string} url - The URL of the wasm file to instantiate.
     * @returns {Promise<SuperDB>} A promise that resolves to a new `SuperDB` instance.
     */
    static instantiate(url) {
        return this.fetchCompressed(url)
            .then((resp) => this.createInstance(resp))
            .then((instance) => new SuperDB(instance));
    }

    /**
     * @private
     */
    static fetchCompressed(url) {
        const headers = { "Content-Type": "application/wasm" };
        const gzip = new DecompressionStream("gzip");
        return fetch(url)
            .then((resp) => resp.blob())
            .then((blob) => blob.stream().pipeThrough(gzip))
            .then((stream) => new Response(stream, { headers }));
    }

    /**
     * @private
     */
    static async createInstance(response) {
        const go = new Go();
        const env = go.importObject;
        const wasm = await WebAssembly.instantiateStreaming(response, env);
        go.run(wasm.instance);
        return __go_wasm__;
    }

    /**
     * @private
     */
    constructor(instance) {
        this.instance = instance;
    }

    /**
     * Executes a query using the provided options and returns the result.
     * @async
     * @param {Object} opts - The options for the query.
     * @param {string} [opts.query] - The ZQ program to execute.
     * @param {string | ReadableStream} [opts.input] - The input data for the query.
     * @param {'auto' | 'arrows' | 'csv' | 'json' | 'line' | 'parquet' | 'tsv' | 'vng' | 'zeek' | 'zjson' | 'zng' | 'zson'} [opts.inputFormat] - The format of the input data.
     * @param {'auto' | 'arrows' | 'csv' | 'json' | 'line' | 'parquet' | 'tsv' | 'vng' | 'zeek' | 'zjson' | 'zng' | 'zson'} [opts.outputFormat] - The desired format of the output data.
     * @returns {Promise<any[]>} A promise that resolves to the processed query result.
     */
    run(args) {
        return this.instance.zq({
            input: args.input,
            inputFormat: args.inputFormat,
            program: args.query,
            outputFormat: args.outputFormat,
        });
    }

    /**
     * Parses the given query string and returns the result.
     * @param {string} query - The query string to parse.
     * @returns {any} The parsed result.
     */
    parse(query) {
        return this.instance.parse(query);
    }
}

let db;

const superdb_wasm_url = document.location.origin + '/superdb.wasm'

SuperDB.instantiate(superdb_wasm_url).then((instance) => {
    db = instance;
});

async function superdb(...args) {
    await waitFor(() => db);
    try {
        return await db.run(...args);
    } catch (e) {
        return e.toString();
    }
}


function waitFor(func) {
    return new Promise((resolve) => {
        function check() {
            if (func()) resolve();
            setTimeout(check, 25);
        }
        check();
    });
}

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

    const node = document.createElement('div');
    node.innerHTML = html;
    pre.replaceWith(node);
    SuperPlayground.setup(node);
}
