export class AriaTabs {
  static setup(tabList) {
    const instance = new AriaTabs(tabList);

    instance.tabs.forEach((tab) => {
      tab.addEventListener('click', (e) => instance.onTabClick(tab));
    });

    instance.list.addEventListener('keydown', (e) => {
      if (e.key == 'ArrowRight') instance.moveFocus(1);
      if (e.key == 'ArrowLeft') instance.moveFocus(-1);
    });
  }

  constructor(tabList) {
    this.group = tabList.parentNode;
    this.list = tabList;
    this.tabs = tabList.querySelectorAll(':scope > [role="tab"]');
    this.panels = this.group.querySelectorAll(':scope [role="tabpanel"]');
  }

  panelFor(tab) {
    const id = tab.getAttribute('aria-controls');
    return this.group.querySelector('#' + id);
  }

  onTabClick(targetTab) {
    function startViewTransition(fn) {
      if (document.startViewTransition) {
        document.startViewTransition(fn);
      } else {
        fn();
      }
    }

    startViewTransition(() => {
      this.tabs.forEach((tab) => this.deselect(tab));
      this.select(targetTab);
      this.panels.forEach((panel) => this.hide(panel));
      this.show(this.panelFor(targetTab));
    });
  }

  moveFocus(n) {
    this.advanceFocusIndex(n);
    this.focus(this.tabs[this.focusIndex]);
  }

  focus(targetTab) {
    this.tabs.forEach((tab) => this.disableFocus(tab));
    this.enableFocus(targetTab);
    targetTab.focus();
  }

  disableFocus(el) {
    el.setAttribute('tabindex', -1);
  }

  enableFocus(el) {
    el.setAttribute('tabindex', 0);
  }

  select(el) {
    el.setAttribute('aria-selected', true);
  }

  deselect(el) {
    el.setAttribute('aria-selected', false);
  }

  hide(el) {
    el.setAttribute('hidden', true);
  }

  show(el) {
    el.removeAttribute('hidden');
  }

  advanceFocusIndex(n) {
    let next = this.focusIndex + n;
    if (next >= this.tabs.length) {
      next = 0;
    } else if (next < 0) {
      next = this.tabs.length - 1;
    }
    this.focusIndex = next;
  }

  get focusIndex() {
    return parseInt(this.list.dataset.focusIndex) || 0;
  }

  set focusIndex(n) {
    this.list.dataset.focusIndex = n;
  }
}
