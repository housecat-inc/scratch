// oxlint-disable effecttsgo/async-function -- These DOM-facing APIs must retain their native Promise contracts; migrating their error channels belongs to #636.
// oxlint-disable effecttsgo/global-fetch -- Native browser requests retain Response, AbortSignal, and Promise rejection contracts; Effect error handling is scoped to #636.
import * as Console from "effect/Console";
import * as Effect from "effect/Effect";
import * as Schema from "effect/Schema";
import * as Str from "effect/String";
import { decode } from "../decode";
import { required } from "../required";
const Result = Schema.Struct({
  conversation_id: Schema.optionalKey(Schema.String),
  project_slug: Schema.optionalKey(Schema.String),
});
export function chat_full(_hook: HTMLElement, _swapped: boolean): void {
  (function() {
    const input = document.querySelector<HTMLTextAreaElement>("#chat-input");
    const sendBtn = document.querySelector<HTMLButtonElement>("#send-btn");

    async function sendMessage(directMessage?: string) {
      const message = directMessage || Str.trim(required(input).value);
      if (!message) return;
      required(sendBtn).disabled = true;
      const payload = { message };
      try {
        const resp = await fetch("/shelley/api/conversations/new", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
        if (!resp.ok) throw new Error("HTTP " + resp.status);
        const result = decode(Result)(await resp.json());
        if (result.conversation_id) {
          window.location.href = "/agent/c/" + encodeURIComponent(result.conversation_id);
        }
      } catch (err) {
        Effect.runSync(Console.error("New conversation error:", err));
        required(sendBtn).disabled = false;
      }
    }

    required(sendBtn).addEventListener("click", () => {
      void sendMessage();
    });
    required(input).addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey && !e.altKey && !e.metaKey) {
        e.preventDefault();
        void sendMessage();
      }
    });

    document.querySelectorAll<HTMLElement>("[data-prompt]").forEach((card) => {
      card.addEventListener("click", function() {
        required(input).value = this.getAttribute("data-prompt") ?? "";
        required(input).focus();
      });
    });

    document.querySelectorAll<HTMLButtonElement>("[data-template]").forEach((card) => {
      card.addEventListener("click", function() {
        void (async () => {
          this.disabled = true;
          try {
            const tpl = this.getAttribute("data-template");
            const resp = await fetch("/api/projects/from-template", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ template: tpl }),
            });
            if (!resp.ok) throw new Error("HTTP " + resp.status);
            const result = decode(Result)(await resp.json());
            if (result.conversation_id) {
              window.location.href = "/agent/c/" + encodeURIComponent(result.conversation_id);
            }
          } catch (err) {
            Effect.runSync(Console.error("Template project error:", err));
            this.disabled = false;
          }
        })();
      });
    });

    const autoPrompt = Str.trim(
      required(document.getElementById("chat-welcome")).getAttribute("data-prompt") || "",
    );
    if (autoPrompt) {
      required(input).value = autoPrompt;
      void sendMessage();
    } else {
      required(input).focus();
    }
  })();
}
