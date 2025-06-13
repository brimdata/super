import './golang.js'

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
      .then((resp) => resp.body.pipeThrough(gzip))
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

export async function superdb(...args) {
  function waitFor(func) {
    return new Promise((resolve) => {
      function check() {
        if (func()) resolve();
        setTimeout(check, 25);
      }
      check();
    });
  }

  await waitFor(() => db);
  try {
    return await db.run(...args);
  } catch (e) {
    return e.toString();
  }
}

let db;

SuperDB.instantiate(document.location.origin + '/superdb.wasm')
  .then((instance) => db = instance);
