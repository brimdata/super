'use strict';

const fs = require('fs');
const zql = require('../zql')

let startRule = 'start';

function show(obj) {
    console.log(JSON.stringify(obj, null, 4));
}

function wrap(e) {
    return { op: 'Error', error: e };
}

function parse_query(line) {
    try {
        return zql.parse(line, {startRule});
    } catch (e) {
        return wrap(e);
    }
}

let filename = '/dev/stdin';
let argv = process.argv.slice(2);

while (argv.length > 0) {
    if (argv[0] === "-e" && argv.length > 1) {
        startRule = argv[1];
        argv = argv.slice(2);
    } else {
        filename = argv.shift();
    }
}

let zql_src;
try {
    zql_src = fs.readFileSync(filename, 'utf8');
} catch (e) {
    show(wrap(e));
    process.exit(1);
}
show(parse_query(zql_src));
