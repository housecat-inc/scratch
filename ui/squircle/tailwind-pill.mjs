import plugin from "tailwindcss/plugin";
// By path into ui's install, not by package name: the standalone tailwindcss
// binary that loads this plugin cannot resolve the name from inside a plugin
// file ("Cannot find module '../dist/babel.cjs'"). squircle is a production
// dependency of ui, so a deploy's production-only install has the path.
import upstream from "../node_modules/@klinking/squircle/dist/tailwind-pill/index.mjs";

// Responsive shapes must carry their own decoration defaults and suppression;
// a bare .squircle-pill selector does not match docked:squircle-pill.
export default plugin((api) => {
  upstream().handler({
    ...api,
    addBase(rules) {
      api.addBase({
        ...rules,
        "@property --squircle-pill-native-border-color": { syntax: '"*"', inherits: "false" },
        "@property --squircle-pill-native-outline-color": { syntax: '"*"', inherits: "false" },
      });
    },
    addUtilities(rules) {
      const shape = rules[".squircle-pill"];
      shape[":where(&)"]["--squircle-pill-decorated"] = "none";
      shape[":where(&)"].position = "relative";
      for (const [selector, declarations] of Object.entries(shape)) {
        if (!selector.startsWith(":where(:root[")) continue;
        delete declarations.position;
        if (selector.includes("data-squircle-pill-worklet")) {
          delete shape[selector];
          shape[selector.replaceAll(" &", " &:not(a[href], a[href] *)")] = declarations;
        }
        Object.assign(declarations, {
          "--squircle-pill-native-border-color": "transparent",
          "--squircle-pill-native-outline-color": "transparent",
          "--tw-ring-shadow": "0 0 #0000 !important",
          "--tw-ring-offset-shadow": "0 0 #0000 !important",
          "--tw-inset-ring-shadow": "0 0 #0000 !important",
        });
      }
      api.addUtilities(rules);
    },
  });
});
