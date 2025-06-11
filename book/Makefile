all: super-playground-bundle.js src/superdb.wasm wasm_exec.js

node_modules:
	npm install

super-playground-bundle.js: super-playground.js editor.js node_modules
	./node_modules/.bin/esbuild --bundle --outfile=$@ $<

wasm_exec.js:
	cp $$(go env GOROOT)/lib/wasm/$@ .

src/superdb.wasm:
	GOPATH=$(PWD) GOOS=js GOARCH=wasm \
		go install github.com/brimdata/superdb-wasm@8a6590f3ead8d0dda0709e89ab41d4c18eea9178
	gzip -c9 bin/js_wasm/superdb-wasm > $@

