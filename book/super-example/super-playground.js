import {Editor} from './editor';
import {super_} from './super';

export class SuperPlayground {
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
    this.result.value = await super_({
      query: this.query.value,
      input: this.input.value
    });
  }
}
