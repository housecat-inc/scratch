import plugin from "tailwindcss/plugin";
// By path into ui's install, not by package name: the standalone tailwindcss
// binary that loads this plugin cannot resolve the name from inside a plugin
// file ("Cannot find module '../dist/babel.cjs'"). squircle is a production
// dependency of ui, so a deploy's production-only install has the path.
import upstream from "../node_modules/@klinking/squircle/dist/tailwind-pill-border/index.mjs";

// Mirror decoration values independently of the shape's responsive prefix.
// The active shape suppresses native painting; ordinary controls retain it.
export default plugin((api) => {
  upstream().handler({
    ...api,
    matchUtilities(utilities, options) {
      api.matchUtilities(
        Object.fromEntries(
          Object.entries(utilities).map(([name, emit]) => [name, (...args) => {
            const rules = emit(...args);
            const declarations = rules["&:is(.squircle-pill)"] ?? {};
            const color = declarations["--squircle-pill-border-color"];
            const outline = declarations["--squircle-pill-outline-color"];
            if (color) declarations["border-color"] = `var(--squircle-pill-native-border-color, ${color})`;
            if (outline || declarations["--squircle-pill-outline-width"]) {
              declarations["outline-color"] = `var(--squircle-pill-native-outline-color, ${outline ?? "currentColor"})`;
            }
            return declarations;
          }]),
        ),
        options,
      );
    },
  });
});
