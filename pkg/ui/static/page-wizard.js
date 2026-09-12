(() => {
  const form = document.getElementById('page-wizard');
  if (!form) return;
  const steps = [...form.querySelectorAll('[data-step]')];
  const back = document.getElementById('wizard-back');
  const next = document.getElementById('wizard-next');
  const submit = document.getElementById('wizard-submit');
  const progress = [...document.querySelectorAll('[data-wizard-step]')];


  let current = 0;
  function review() {
    const details = [
      ['Page name', form.elements.title.value],
      ['What it should do', form.elements.description.value],
      ['Content, data, and actions', form.elements.details.value || 'Let the agent suggest a starting layout.']
    ];
    const list = document.getElementById('page-review');
    list.replaceChildren();
    for (const [label, value] of details) {
      const term = document.createElement('dt');
      const definition = document.createElement('dd');
      term.textContent = label;
      definition.textContent = value;
      list.append(term, definition);
    }
  }
  function show(focus = true) {
    steps.forEach((step, index) => { step.hidden = index !== current; });
    back.hidden = current === 0;
    next.hidden = current === 2;
    submit.hidden = current !== 2;
    progress.forEach((button, index) => {
      button.disabled = index > current;
      if (index === current) button.setAttribute('aria-current', 'step');
      else button.removeAttribute('aria-current');
    });
    if (current === 2) review();
    if (focus) {
      const heading = steps[current].querySelector('legend');
      heading.tabIndex = -1;
      heading.focus();
    }
  }
  function advance() {
    for (const input of steps[current].querySelectorAll('input, textarea, select')) {
      if (!input.reportValidity()) return;
    }
    current++;
    show();
  }
  progress.forEach((button, index) => {
    button.addEventListener('click', () => { current = index; show(); });
  });
  back.addEventListener('click', () => { current--; show(); });
  next.addEventListener('click', advance);
  form.noValidate = true;
  form.addEventListener('submit', event => {
    if (current !== 2) {
      event.preventDefault();
      advance();
      return;
    }
    for (const input of form.querySelectorAll('input, textarea, select')) {
      if (!input.checkValidity()) {
        event.preventDefault();
        current = Number(input.closest('[data-step]').dataset.step);
        show();
        input.reportValidity();
        return;
      }
    }
    submit.disabled = true;
    submit.textContent = 'Opening chat…';
  });
  window.addEventListener('pageshow', () => {
    submit.disabled = false;
    submit.textContent = 'Build page →';
  });
  show(false);
})();
