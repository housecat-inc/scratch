import { required } from "../required";
export function disclosure(_hook: HTMLElement, _swapped: boolean): void {
  required(window.htmx).registerExtension("preserve-open", {
    htmx_before_morph_attr(element: Element, detail: { attrName: string }): boolean | void {
      if (detail.attrName === "open" && element.matches("details[data-preserve-open]")) {
        return false;
      }
    },
  });
}
