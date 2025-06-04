import {Editor} from './editor';
import {super_} from './super';

export class SuperPlayground {
  static setup(node, onQueryOrInputChange) {
    const playground = new SuperPlayground(node);
    node.__super_playground__ = playground;
    playground.setup(onQueryOrInputChange);
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

  setup(onQueryOrInputChange) {
    this.input = new Editor({
      node: this.node.querySelector('.input pre'),
      onChange: () => {
        onQueryOrInputChange?.call(null, this.query.value, this.input.value);
        this.run();
      }
    });
    this.query = new Editor({
      node: this.node.querySelector('.query pre'),
      onChange: () => {
        onQueryOrInputChange?.call(null, this.query.value, this.input.value);
        this.run();
      },
      language: 'sql'
    });
    this.result = new Editor({
      node: this.node.querySelector('.result pre')
    });
    onQueryOrInputChange?.call(null, this.query.value, this.input.value);
    this.run();
  }

  teardown() {
    this.input.teardown();
    this.query.teardown();
    this.result.teardown();
  }

  async run() {
    this.result.value = await super_({
      query: this.query.value,
      input: this.input.value
    });
  }
}
