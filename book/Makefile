all: src/superdb.wasm super-playground/super-playground-bundle.js

src/superdb.wasm:
	GOOS=js GOARCH=wasm go build -o $@ super-playground/main.go
	gzip -9 $@
	mv $@.gz $@

super-playground/super-playground-bundle.js: node_modules super-playground/wasm_exec.js super-playground/*.js
	./node_modules/.bin/esbuild --bundle --outfile=$@ super-playground/super-playground.js

super-playground/wasm_exec.js:
	cp $$(go env GOROOT)/lib/wasm/$(@F) $@

node_modules:
	npm install


