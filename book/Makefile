js_bundle := super-example.bundle.js
wasm := src/super.wasm

all: $(js_bundle) $(wasm)

$(js_bundle): node_modules super-example/wasm_exec.js super-example/*.js
	./node_modules/.bin/esbuild --bundle --outfile=$@ super-example/super-example.js

super-example/wasm_exec.js:
	cp $$(go env GOROOT)/lib/wasm/$(@F) $@

node_modules:
	npm install

$(wasm): super-example/main.go
	GOOS=js GOARCH=wasm go build -o $@ $<
