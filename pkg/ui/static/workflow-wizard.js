(() => {
  const form = document.getElementById('workflow-wizard');
  if (!form) return;
  const steps = [...form.querySelectorAll('[data-step]')];
  const back = document.getElementById('wizard-back');
  const next = document.getElementById('wizard-next');
  const submit = document.getElementById('wizard-submit');
  const progress = [...document.querySelectorAll('[data-wizard-step]')];
  const trigger = form.elements.trigger;
  const timezone = form.elements.timezone;
  let current = 0;
  timezone.value = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  function updateTrigger() {
    const scheduled = trigger.value === 'schedule';
    document.getElementById('workflow-zone').hidden = !scheduled;
    timezone.required = scheduled;
    document.getElementById('workflow-timing-label').textContent = scheduled ? 'Describe the schedule' : 'What event should start it, and where?';
    form.elements.timing.placeholder = scheduled ? 'Every weekday at 9 AM' : 'When new feedback arrives in our support inbox';
  }
  function review() {
    const details = [
      ['What it should do', form.elements.description.value],
      ['Trigger', trigger.value === 'schedule' ? 'On a schedule' : 'From an external trigger'],
      ['When', form.elements.timing.value]
    ];
    if (trigger.value === 'schedule') details.push(['Timezone', timezone.value]);
    const list = document.getElementById('workflow-review');
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
  trigger.addEventListener('change', updateTrigger);
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
    submit.textContent = 'Build workflow →';
  });
  updateTrigger();
  show(false);
})();
