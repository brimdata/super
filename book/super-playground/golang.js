import './wasm_exec.js'

function wrapper(goFunc) {
  return (...args) => {
    const result = goFunc.apply(undefined, args);
    if (result.error instanceof Error) {
      throw result.error;
    }
    return result.result;
  };
}

// For github.com/teamortix/golang-wasm/wasm.
globalThis.__go_wasm__ = {
  __wrapper__: wrapper,
};
