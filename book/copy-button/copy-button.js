(() => {
  for (const pre of document.querySelectorAll('pre:has(>code)')) {
    addCopyButton(pre, (pre) => pre.querySelector('code').textContent);
  }

  for (const el of document.querySelectorAll('.editor')) {
    addCopyButton(el, (el) => {
      const lines = [];
      for (const line of el.querySelectorAll('.cm-line')) lines.push(line.textContent);
      return lines.join('\n');
    });
  }

  function addCopyButton(pre, getContent) {
    const button = document.createElement('button');
    button.setAttribute('class', 'copy-button');
    button.textContent = 'Copy';
    pre.append(button);

    button.addEventListener('click', async () => {
      const content = getContent(pre);
      await navigator.clipboard.writeText(content);
      button.setAttribute('aria-pressed', true);
      button.textContent = 'Copied';

      setTimeout(() => {
        button.removeAttribute('aria-pressed');
        button.textContent = 'Copy';
      }, 2500);
    });
  }
})();
