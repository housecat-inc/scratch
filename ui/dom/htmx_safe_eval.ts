// htmx compiles hx-on and hx-live expressions with Function and
// AsyncFunction, which a CSP without 'unsafe-eval' blocks. This swaps them,
// through htmx's initSecurity hook, for scripts carrying the page's nonce,
// compiled once per expression, as htmx's own hx-csp safeEval does. It is a
// classic script the layout loads right after htmx, so the expressions run
// even when ui.js does not.
type Compiler = { call: (thisArg: unknown, ...values: unknown[]) => unknown };
type Constructor = (...keysAndBody: string[]) => Compiler;
type SecurityApi = { initSecurity: (scriptPolicy: null, fn: Constructor, asyncFn: Constructor) => void };
type Htmx = { registerExtension: (name: string, extension: object) => void };
type Compiled = (thisArg: unknown, values: unknown[]) => unknown;

export function nonceConstructor(
  nonce: string,
  isAsync: boolean,
  cache: Map<string, Compiled>,
  doc: Document = document,
): Constructor {
  return function(...keysAndBody: string[]): Compiler {
    const body = keysAndBody.pop() ?? "";
    const params = keysAndBody.join(",");
    const key = `${isAsync ? "async" : "sync"}|${params}|${body}`;
    return {
      call: (thisArg, ...values) => {
        let compiled = cache.get(key);
        if (!compiled) {
          const name = `__housecatEval${String(cache.size + 1)}`;
          const script = doc.createElement("script");
          script.nonce = nonce;
          script.textContent = `window.${name} = ${isAsync ? "async " : ""}function(${params}) { ${body} }`;
          doc.head.append(script);
          script.remove();
          const scope = doc.defaultView;
          const found: unknown = scope ? Reflect.get(scope, name) : undefined;
          if (scope) Reflect.deleteProperty(scope, name);
          if (typeof found !== "function") throw new Error("htmx expression blocked by the Content-Security-Policy");
          compiled = (self, args) => {
            const result: unknown = Reflect.apply(found, self, args);
            return result;
          };
          cache.set(key, compiled);
        }
        return compiled(thisArg, values);
      },
    };
  };
}

export function install(htmx: Htmx, nonce: string): void {
  const cache = new Map<string, Compiled>();
  const extension = {
    init: (api: SecurityApi) =>
      api.initSecurity(null, nonceConstructor(nonce, false, cache), nonceConstructor(nonce, true, cache)),
  };
  htmx.registerExtension("housecat-safe-eval", extension);
}

const script = document.currentScript;
const pageNonce = script instanceof HTMLScriptElement ? script.nonce : "";
if (window.htmx && pageNonce) install(window.htmx, pageNonce);
