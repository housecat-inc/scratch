type Validated = Element & {
  readonly form: HTMLFormElement | null;
  readonly validity: ValidityState;
  readonly willValidate: boolean;
};

export function validated(el: unknown): el is Validated {
  return el instanceof Element && "validity" in el && "willValidate" in el;
}

export function firstInvalid(el: Validated): Element | null {
  if (!el.form) return el;
  return Array.from(el.form.elements).find((each) => validated(each) && each.willValidate && !each.validity.valid)
    ?? null;
}

const prior = new WeakMap<HTMLElement, { describedBy: string | null; invalid: string | null }>();

export function reportRequired(root: Element, controls: HTMLElement[]): void {
  const error = root.querySelector<HTMLElement>("[data-slot=control-validation-error]");
  if (!error) return;
  error.hidden = false;
  error.textContent = "Please select an option.";
  controls.forEach((control) => {
    if (!prior.has(control)) {
      prior.set(control, {
        describedBy: control.getAttribute("aria-describedby"),
        invalid: control.getAttribute("aria-invalid"),
      });
    }
    const ids = new Set((control.getAttribute("aria-describedby") ?? "").split(/\s+/).filter(Boolean));
    ids.add(error.id);
    control.setAttribute("aria-describedby", [...ids].join(" "));
    control.setAttribute("aria-invalid", "true");
  });
}

export function clearRequired(root: Element, controls: HTMLElement[]): void {
  const error = root.querySelector<HTMLElement>("[data-slot=control-validation-error]");
  if (error) {
    error.hidden = true;
    error.textContent = "";
  }
  controls.forEach((control) => {
    const original = prior.get(control);
    if (!original) return;
    const restore = (name: string, value: string | null): void => {
      if (value === null) control.removeAttribute(name);
      else control.setAttribute(name, value);
    };
    restore("aria-describedby", original.describedBy);
    restore("aria-invalid", original.invalid);
    prior.delete(control);
  });
}

export function watchDisabled(rootSelector: string, triggerSelector: string, valueSelector: string): void {
  const observer = new MutationObserver((changes) => {
    changes.forEach(({ target }) => {
      if (!(target instanceof HTMLElement) || !target.matches(triggerSelector)) return;
      const root = target.closest(rootSelector);
      if (!root) return;
      const disabled = target.matches(":disabled");
      root.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>(valueSelector).forEach((input) => {
        if (input.disabled !== disabled) input.disabled = disabled;
      });
      if (disabled) clearRequired(root, [target, ...root.querySelectorAll<HTMLElement>("[data-slot=combobox-input]")]);
    });
  });
  observer.observe(document, { attributes: true, attributeFilter: ["disabled"], subtree: true });
}
