import * as Arr from "effect/Array";
import * as Rec from "effect/Record";
export type PaletteData = {
  checked?: string | null;
  id?: string;
  into?: string | null;
  label?: string | null;
  query?: string;
  src?: string | null;
  target?: string;
  text?: string;
};
export type PaletteAction = (data?: PaletteData) => unknown;
type Actions = Record<string, PaletteAction | undefined>;
type Layer = { root: Element; actions: Actions };
declare global {
  interface Window {
    paletteActions: Actions;
    HousecatPaletteActions: {
      get: (name: string) => PaletteAction | null;
      register: (root: Element | null, actions: Actions) => void;
    };
  }
}
const provided: Partial<Window> = typeof window === "undefined" ? {} : window;
export const paletteActions: Actions = provided.paletteActions ?? {};
export const paletteAPI = (() => {
  // paletteActions is the base map, for actions that hold wherever
  // they were installed. A part of the page that brings actions of its own
  // registers them with its root element: an action registered with a root
  // applies only while that root is in the document, so a part that an
  // htmx swap takes away takes its actions with it and the base map's
  // answer again. get looks through the newest live layer first.
  let layers: Layer[] = [];
  function live(layer: Layer) {
    return layer.root.isConnected;
  }
  function has(actions: Actions, name: string) {
    return Rec.has(actions, name) && typeof actions[name] === "function";
  }
  function register(root: Element | null, actions: Actions) {
    layers = Arr.filter(layers, live);
    if (root) layers.push({ actions, root });
  }
  function get(name: string) {
    for (let i = layers.length - 1; i >= 0; i--) {
      if (live(layers[i]) && has(layers[i].actions, name)) return layers[i].actions[name] ?? null;
    }
    return has(paletteActions, name) ? (paletteActions[name] ?? null) : null;
  }

  return { get, register };
})();
if (typeof window !== "undefined") window.paletteActions = paletteActions;
if (typeof window !== "undefined") window.HousecatPaletteActions = paletteAPI;
