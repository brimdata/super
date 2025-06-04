import {minimalSetup, EditorView} from 'codemirror';
import {sql} from '@codemirror/lang-sql';
import {oneDark} from '@codemirror/theme-one-dark';

export class Editor {
  constructor(options = {}) {
    this.language = options.language;
    this.onChange = options.onChange;
    this.originalNode = options.node;
    this.parentNode = this.originalNode.parentNode;
    this.initialContent = this.originalNode.textContent;
    this.originalNode.remove();
    this.view = new EditorView({
      parent: this.parentNode,
      doc: this.initialContent,
      extensions: [minimalSetup, this.updater, oneDark, this.syntax]
    });
  }

  teardown() {
    this.view.destroy();
    this.parentNode.append(this.originalNode);
  }

  get syntax() {
    if (this.language === 'sql') return sql();
    else return [];
  }

  get value() {
    return this.view.state.doc.toString();
  }

  set value(text) {
    this.view.dispatch({
      changes: {from: 0, to: this.view.state.doc.length, insert: text.trim()}
    });
  }

  get updater() {
    return EditorView.updateListener.of((update) => {
      if (update.docChanged) this.handleUpdate(this.view.state.doc.toString());
    });
  }

  handleUpdate(value) {
    this.onChange?.call(null, value);
  }
}
